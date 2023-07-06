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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MySQLSpec defines the desired state of MySQL
type MySQLSpec struct {
	Authentication MySQLAuthenticationSpec `json:"authentication"`
	// Version is the MySQL instance version that must be run as Pods.
	// It refers to the Docker Hub available tags: https://hub.docker.com/_/mysql/tags
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
}

type MySQLAuthenticationSpec struct {
	// Assign the root password for the MySQL instance.
	RootPassword string `json:"rootPassword"`
}

// MySQLStatus defines the observed state of MySQL
type MySQLStatus struct {
	RootPassword string `json:"rootPassword"`
	// Check if the required resources have been provisioned.
	Initialized bool `json:"initialized"`
	// The IP address on which the MySQL instance is listening to.
	Address string `json:"address"`
	// Check if the MySQL instance is up and running.
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Address",type=string,JSONPath=".status.address"
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MySQL is the Schema for the mysqls API
type MySQL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLSpec   `json:"spec,omitempty"`
	Status MySQLStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MySQLList contains a list of MySQL
type MySQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQL `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MySQL{}, &MySQLList{})
}
