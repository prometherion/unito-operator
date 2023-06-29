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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	k8sv1beta1 "github.com/prometherion/unito-operator/api/v1beta1"
)

// StudentReconciler reconciles a Student object
type StudentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8s.unito.it,resources=students,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.unito.it,resources=students/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.unito.it,resources=students/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Student object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *StudentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	student := k8sv1beta1.Student{}
	if err := r.Client.Get(ctx, req.NamespacedName, &student); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("object has been deleted")

			return ctrl.Result{}, nil
		}

		logger.Error(err, "unable to retrieve student")

		return ctrl.Result{}, err
	}

	if student.DeletionTimestamp != nil {
		// DELETE UNITO API
		//
		if true {
			controllerutil.RemoveFinalizer(&student, "k8s.unito.it/api")

			return ctrl.Result{}, r.Client.Update(ctx, &student)
		}
	}

	if len(student.Spec.Nickname) == 0 {
		// Interacting with UNITO API and retrieving Student ID
		logger.Info("skipping resource, missing nickname")

		return ctrl.Result{Requeue: true}, nil
	}
	// Creating UNITO API
	if !controllerutil.ContainsFinalizer(&student, "k8s.unito.it/api") {
		// CALL UNITO API
		controllerutil.AddFinalizer(&student, "k8s.unito.it/api")

		return ctrl.Result{}, r.Client.Update(ctx, &student)
	}

	logger.Info("reconciling student", "nickname", student.Spec.Nickname)

	deployment := appsv1.Deployment{}
	deployment.SetNamespace(student.Namespace)
	deployment.SetName(fmt.Sprintf("unito-%s-%s", student.UID, student.Name))

	operationResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"student":  student.Name,
				"operator": "unito",
			},
		}
		deployment.Spec.Template.ObjectMeta.Labels = map[string]string{
			"student":  student.Name,
			"operator": "unito",
		}
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:                     "nginx",
				Image:                    "nginx",
				ImagePullPolicy:          corev1.PullAlways,
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			},
		}

		r.Client.Scheme().Default(&deployment)

		return controllerutil.SetControllerReference(&student, &deployment, r.Client.Scheme())
	})
	if err != nil {
		logger.Error(err, "unable to create or update student's Deployment")

		return ctrl.Result{}, err
	}

	logger.Info("deployment reconciliation completed", "operation", operationResult)

	svc := corev1.Service{}
	svc.SetNamespace(student.Namespace)
	svc.SetName(fmt.Sprintf("unito-%s-%s", student.UID, student.Name))

	operationResult, err = controllerutil.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:     "http",
				Protocol: "TCP",
				Port:     80,
				TargetPort: intstr.IntOrString{
					IntVal: 80,
				},
			},
		}
		svc.Spec.Selector = deployment.Spec.Selector.MatchLabels

		return controllerutil.SetControllerReference(&student, &svc, r.Client.Scheme())
	})
	if err != nil {
		logger.Error(err, "unable to create or update student's Service")

		return ctrl.Result{}, err
	}

	logger.Info("service reconciliation completed", "operation", operationResult)

	if deployment.Status.UnavailableReplicas > 0 {
		logger.Info("cannot exec into the Pod")
		student.Status.Initialized = false

		if err = r.Client.Status().Update(ctx, &student); err != nil {
			logger.Error(err, "unable to update status")

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	student.Status.Initialized = true
	student.Status.Acceptance = "Accepted"
	if err = r.Client.Status().Update(ctx, &student); err != nil {
		logger.Error(err, "unable to update status")

		return ctrl.Result{}, err
	}

	logger.Info("reconciliation completed")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StudentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1beta1.Student{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
