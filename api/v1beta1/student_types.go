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

// StudentSpec defines the desired state of Student
type StudentSpec struct {
	// Surname is the student surname.
	Surname string `json:"surname"`
	// BirthDate is referring to the YYYY-MM-DD birthdate of the student.
	BirthDate string `json:"birthDate"`
	// CourseYear is the academic year the student is attending.
	// +kubebuilder:default="2022-2023"
	CourseYear string `json:"courseYear,omitempty"`
	// Nickname refers to the nickname of the student.
	Nickname string `json:"nickname,omitempty"`
}

// StudentStatus defines the observed state of Student
type StudentStatus struct {
	Initialized bool `json:"initialized"`
	// +kubebuilder:validation:Enum=Rejected;Accepted
	Acceptance string `json:"acceptance"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Nickname",type=string,JSONPath=`.spec.nickname`
//+kubebuilder:printcolumn:name="Acceptance",type=string,JSONPath=`.status.acceptance`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Student is the API to manage and control UNITO students.
type Student struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StudentSpec   `json:"spec,omitempty"`
	Status StudentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StudentList contains a list of Student
type StudentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Student `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Student{}, &StudentList{})
}
