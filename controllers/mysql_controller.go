/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	databasev1beta1 "github.com/prometherion/unito-operator/api/v1beta1"
)

// MySQLReconciler reconciles a MySQL object
type MySQLReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.unito.it,resources=mysqls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.unito.it,resources=mysqls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.unito.it,resources=mysqls/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch

func (r *MySQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("handling resource")

	db := databasev1beta1.MySQL{}
	if err := r.Client.Get(ctx, req.NamespacedName, &db); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("object has been deleted")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}
	// Defining the MySQL Pod
	pod := corev1.Pod{}
	pod.Name = db.Name
	pod.Namespace = db.Namespace
	// Generating the Pod and getting the password
	password, or, err := r.createPod(ctx, &db, &pod)
	if err != nil {
		logger.Error(err, "cannot create the MySQL instance pod")

		return ctrl.Result{}, err
	}

	logger.Info("pod reconciliation completed", "result", or)

	db.Status.Initialized = true
	if err = r.updateDatabaseInstanceStatus(ctx, &db); err != nil {
		logger.Error(err, "unable to save MySQL instance status")
	}
	// Defining the MySQL Service
	svc := corev1.Service{}
	svc.Name = db.Name
	svc.Namespace = db.Namespace

	if or, err = r.createService(ctx, &db, &svc); err != nil {
		logger.Error(err, "cannot create the MySQL instance service")

		return ctrl.Result{}, err
	}

	logger.Info("service reconciliation completed", "result", or)

	if len(svc.Spec.ClusterIP) == 0 {
		logger.Info("waiting for ClusterIP for the MySQL instance")

		return ctrl.Result{}, nil
	}

	db.Status.Address = svc.Spec.ClusterIP
	if err = r.updateDatabaseInstanceStatus(ctx, &db); err != nil {
		logger.Error(err, "unable to save MySQL instance status")
	}
	// Update the Database status
	db.Status.RootPassword = password

	if err = r.updateDatabaseInstanceStatus(ctx, &db); err != nil {
		logger.Error(err, "unable to save MySQL instance status")
	}
	// Update the readiness status
	db.Status.Ready = pod.Status.Phase == corev1.PodRunning

	if err = r.updateDatabaseInstanceStatus(ctx, &db); err != nil {
		logger.Error(err, "unable to save MySQL instance status")
	}

	logger.Info("reconciliation completed")

	return ctrl.Result{}, nil
}

func (r *MySQLReconciler) updateDatabaseInstanceStatus(ctx context.Context, db *databasev1beta1.MySQL) error {
	return r.Client.Status().Update(ctx, db)
}

func (r *MySQLReconciler) createService(ctx context.Context, db *databasev1beta1.MySQL, svc *corev1.Service) (operationResult controllerutil.OperationResult, err error) {
	return ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "mysql",
				Protocol:   "TCP",
				Port:       3306,
				TargetPort: intstr.FromInt(3306),
			},
		}
		svc.Spec.Selector = map[string]string{
			"unito.it/db":   "mysql",
			"unito.it/name": db.Name,
		}

		return controllerutil.SetControllerReference(db, svc, r.Scheme)
	})
}

func (r *MySQLReconciler) createPod(ctx context.Context, db *databasev1beta1.MySQL, pod *corev1.Pod) (password string, operationResult controllerutil.OperationResult, err error) {
	logger := log.FromContext(ctx)

	if len(db.Spec.Authentication.RootPassword) > 0 && db.Spec.Authentication.RootPassword != db.Status.RootPassword {
		if err = r.Client.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "unable to clean-up Pod upon root password change")

			return "", controllerutil.OperationResultNone, err
		}
	}

	operationResult, err = ctrl.CreateOrUpdate(ctx, r.Client, pod, func() error {
		if len(pod.Spec.Containers) == 0 {
			pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{})
		}

		pod.Spec.Containers[0].Name = "db"
		pod.Spec.Containers[0].Image = fmt.Sprintf("docker.io/mysql:%s", db.Spec.Version)
		pod.SetLabels(map[string]string{
			"unito.it/project": "operator",
			"unito.it/db":      "mysql",
			"unito.it/name":    db.Name,
		})
		// Environment variables
		if len(pod.Spec.Containers[0].Env) == 0 {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{})
		}

		pod.Spec.Containers[0].Env[0].Name = "MYSQL_ROOT_PASSWORD"
		switch {
		case len(db.Spec.Authentication.RootPassword) > 0:
			// A new DB instance, with a user-input password
			password = db.Spec.Authentication.RootPassword
		case len(pod.Spec.Containers[0].Env[0].Value) == 0:
			// A new DB instance, with a random password generated by the Operator
			password = uuid.New().String()
		default:
			password = pod.Spec.Containers[0].Env[0].Value
		}
		// Assigning the password to the MySQL pods
		pod.Spec.Containers[0].Env[0].Value = password

		return controllerutil.SetControllerReference(db, pod, r.Scheme)
	})

	return password, operationResult, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *MySQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1beta1.MySQL{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
