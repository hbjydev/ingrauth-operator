/*
Copyright 2022.

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

package v1alpha1

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressSubSpec defines the desired state of the _generated_ Ingress resource.
type IngressSubSpec struct {
	// Metadata defines the labels and annotations to attach to the generated
	// Ingress resource.
	Metadata IngressSubSpecMeta `json:"metadata"`

	// Spec is the IngressSpec object required to set up the ingress itself.
	Spec networkingv1.IngressSpec `json:"spec"`
}

// IngressSubSpecMeta is the metadata specification that is used when templating
// the generated Ingress resource.
type IngressSubSpecMeta struct {
	// Labels are the labels to inject into the Ingress resource.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are the annotations to inject into the Ingress resource.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressSpec defines the desired state of Ingress
type IngressSpec struct {
	// Template defines the template for generating the networking/v1/Ingress
	// object.
	Template IngressSubSpec `json:"template"`

	// UserSelector is a set of labels that can be used to select User objects.
	UserSelector map[string]string `json:"userSelector"`
}

// IngressStatus defines the observed state of Ingress
type IngressStatus struct {
	// IngressResource is the name of the generated networking/v1/Ingress
	// resource.
	IngressResource string `json:"ingressResource"`

	// AuthSecret is the name of the v1/Secret resource managed by this
	// Ingress.
	AuthSecret string `json:"authSecret"`

	// Users is a list of the User objects associated with this Ingress.
	//  users:
	//    "auth-sample": "1"
	Users map[string]string `json:"users"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Ingress is the Schema for the ingresses API
type Ingress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressSpec   `json:"spec,omitempty"`
	Status IngressStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IngressList contains a list of Ingress
type IngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ingress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ingress{}, &IngressList{})
}
