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
	errors "sigs.k8s.io/cluster-api/errors"
)

const (
	MachineFinalizer = "infrastructure.cluster.x-k8s.io/libvirtmachine"
)

type LibvirtMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID        *string            `json:"providerID,omitempty"`
	Name              string             `json:"name"`
	CPU               int                `json:"cpu"`
	Memory            int                `json:"memory"`
	DiskImage         []DiskImage        `json:"diskImage"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
	Autostart         string             `json:"autostart"`
}

type DiskImage struct {
	PoolName     string `json:"poolName"`
	BaseVolumeID string `json:"baseVolumeID"`
	VolumeSize   string `json:"volumeSize"`
}

type NetworkInterface struct {
	NetworkType string `json:"networkType"`
	Network     string `json:"network"`
	Bridge      string `json:"bridge"`
	MACAddress  string `json:"macAddress,omitempty"`
}

// LibvirtMachineStatus defines the observed state of LibvirtMachine.
type LibvirtMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// InstanceState is the state of the server for this machine.
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions define the current service state of the LibvirtMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// LibvirtMachine is the Schema for the libvirtmachines API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=libvirtmachines,scope=Namespaced,categories=cluster-api,shortName=hcma
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this LibvirtMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this LibvirtMachine"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.instanceState",description="Phase of LibvirtMachine"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Libvirtmachine"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +k8s:defaulter-gen=true

// LibvirtMachine is the Schema for the libvirtmachines API.
type LibvirtMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LibvirtMachineSpec   `json:"spec,omitempty"`
	Status LibvirtMachineStatus `json:"status,omitempty"`
}

// LibvirtMachineSpec returns a DeepCopy.
func (r *LibvirtMachine) LibvirtMachineSpec() *LibvirtMachineSpec {
	return r.Spec.DeepCopy()
}

// GetConditions returns the observations of the operational state of the LibvirtMachine resource.
func (r *LibvirtMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the LibvirtMachine to the predescribed clusterv1.Conditions.
func (r *LibvirtMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// LibvirtMachineList contains a list of LibvirtMachine.
type LibvirtMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LibvirtMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &LibvirtMachine{}, &LibvirtMachineList{})
}
