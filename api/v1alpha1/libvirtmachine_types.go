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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LibvirtMachineSpec defines the desired state of LibvirtMachine.
type LibvirtMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Number of CPU cores to assign to the virtual machine
	Cpu int `json:"cpu"`
	// Amount of memory to assign to the virtual machine
	Memory int `json:"memory"`
	// Add a cloud init template to the virtual machine
	CloudInit *CloudInit `json:"cloudInit,omitempty"`
	// Specifiy a volume block for the virtual machine
	Volume *Volume `json:"volume"`
	// UUID of the network to attach the virtual machine to
	NetworkUUID string `json:"networkUUID"`
	// Enable autostart for the virtual machine
	Autostart bool `json:"autostart"`
}

type CloudInit struct {
	// The cloud init template
	Template string `json:"template"`
}

type Volume struct {
	PoolName     string `json:"poolName"`
	BaseVolumeID string `json:"baseVolumeID"`
	VolumeName   string `json:"volumeName"`
	VolumeSize   int    `json:"volumeSize"`
}

// LibvirtMachineStatus defines the observed state of LibvirtMachine.
type LibvirtMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LibvirtMachine is the Schema for the libvirtmachines API.
type LibvirtMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LibvirtMachineSpec   `json:"spec,omitempty"`
	Status LibvirtMachineStatus `json:"status,omitempty"`
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
