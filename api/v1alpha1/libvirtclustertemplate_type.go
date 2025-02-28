/*
Copyright 2021 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// LibvirtClusterTemplateSpec defines the desired state of LibvirtClusterTemplate.
type LibvirtClusterTemplateSpec struct {
	Template LibvirtClusterTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=Libvirtclustertemplates,scope=Namespaced,categories=cluster-api,shortName=hcclt
// +k8s:defaulter-gen=true

// LibvirtClusterTemplate is the Schema for the Libvirtclustertemplates API.
type LibvirtClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LibvirtClusterTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// LibvirtClusterTemplateList contains a list of LibvirtClusterTemplate.
type LibvirtClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LibvirtClusterTemplate `json:"items"`
}

// LibvirtClusterTemplateResource contains spec for LibvirtClusterSpec.
type LibvirtClusterTemplateResource struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	Spec       LibvirtClusterSpec   `json:"spec"`
}

func init() {
	objectTypes = append(objectTypes, &LibvirtClusterTemplate{}, &LibvirtClusterTemplateList{})
}
