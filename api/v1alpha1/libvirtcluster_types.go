/*
Copyright 2025.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// ClusterFinalizer allows cleaning up resources associated with a LibvirtCluster before deletion is completed.
	ClusterFinalizer = "infrastructure.cluster.x-k8s.io/libvirtcluster"
)

// LibvirtClusterSpec defines the desired state of LibvirtCluster.
type LibvirtClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.port > 0 && self.port < 65536",message="port must be within 1-65535"
	ControlPlaneEndpoint *clusterv1.APIEndpoint `json:"foo,omitempty"`
	LibvirtSecret        LibvirtSecretRef       `json:"libvirtSecretRef"`
}

// LibvirtClusterStatus defines the observed state of LibvirtCluster.
type LibvirtClusterStatus struct {
	// Ready indicates that the cluster is ready.
	// +kubebuilder:default=false
	Ready      bool                 `json:"ready"`
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LibvirtCluster is the Schema for the libvirtclusters API.
type LibvirtCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LibvirtClusterSpec   `json:"spec,omitempty"`
	Status LibvirtClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LibvirtClusterList contains a list of LibvirtCluster.
type LibvirtClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LibvirtCluster `json:"items"`
}

// GetConditions returns the observations of the operational state of the HetznerCluster resource.
func (r *LibvirtCluster) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerCluster to the predescribed clusterv1.Conditions.
func (r *LibvirtCluster) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &LibvirtCluster{}, &LibvirtClusterList{})
}
