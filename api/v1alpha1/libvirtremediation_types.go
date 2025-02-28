package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// LibvirtRemediationSpec defines the desired state of LibvirtRemediation
type LibvirtRemediationSpec struct {
	// Remediation is the remediation to apply.
	Strategy *RemediationStrategy `json:"strategy,omitempty"`
}

// LibvirtRemediationStatus defines the observed state of LibvirtRemediation.
type LibvirtRemediationStatus struct {
	// Phase represents the current phase of machine remediation.
	// E.g. Pending, Running, Done etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// RetryCount can be used as a counter during the remediation.
	// Field can hold number of reboots etc.
	// +optional
	RetryCount int `json:"retryCount,omitempty"`

	// LastRemediated identifies when the host was last remediated
	// +optional
	LastRemediated *metav1.Time `json:"lastRemediated,omitempty"`

	// Conditions defines current service state of the LibvirtRemediation.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=Libvirtremediations,scope=Namespaced,categories=cluster-api,shortName=hcr
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.strategy.timeout",description="Timeout for the remediation"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Phase of the remediation"
// +kubebuilder:printcolumn:name="Last Remediated",type=string,JSONPath=".status.lastRemediated",description="Timestamp of the last remediation attempt"
// +kubebuilder:printcolumn:name="Retry count",type=string,JSONPath=".status.retryCount",description="How many times remediation controller has tried to remediate the node"
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the node"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// LibvirtRemediation is the Schema for the Libvirtremediations API.
type LibvirtRemediation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec LibvirtRemediationSpec `json:"spec,omitempty"`
	// +optional
	Status LibvirtRemediationStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the LibvirtRemediation resource.
func (r *LibvirtRemediation) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the LibvirtRemediation to the predescribed clusterv1.Conditions.
func (r *LibvirtRemediation) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// LibvirtRemediationList contains a list of LibvirtRemediation.
type LibvirtRemediationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LibvirtRemediation `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &LibvirtRemediation{}, &LibvirtRemediationList{})
}
