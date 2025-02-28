package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// LibvirtURIAvailableCondition reports on whether the HCloud Token is available.
	LibvirtURIAvailableCondition clusterv1.ConditionType = "LibvirtURIAvailable"
	// LibvirtSecretUnreachableReason indicates that libvirt secret is unreachable.
	LibvirtSecretUnreachableReason = "LibvirtSecretUnreachable" // #nosec
	// LibvirtURIInvalidReason indicates that credentials for HCloud are invalid.
	LibvirtURIInvalidReason = "LibvirtCredentialsInvalid" // #nosec
)

const (
	// DomainAvailableCondition indicates the instance is in a Running state.
	DomainAvailableCondition clusterv1.ConditionType = "DomainAvailable"
	// DomainTerminatingReason instance is in a terminated state.
	DomainTerminatingReason = "DomainTerminated"
	// DomainStartingReason instance is in a terminated state.
	DomainStartingReason = "DomainStarting"
	// DomainOffReason instance is off.
	DomainOffReason = "DomainOff"
)

const (
	// BootstrapReadyCondition  indicates that bootstrap is ready.
	BootstrapReadyCondition clusterv1.ConditionType = "BootstrapReady"
	// BootstrapNotReadyReason bootstrap not ready yet.
	BootstrapNotReadyReason = "BootstrapNotReady"
)

const (
	// TargetClusterReadyCondition reports on whether the kubeconfig in the target cluster is ready.
	TargetClusterReadyCondition clusterv1.ConditionType = "TargetClusterReady"
	// KubeConfigNotFoundReason indicates that the Kubeconfig could not be found.
	KubeConfigNotFoundReason = "KubeConfigNotFound"
	// KubeAPIServerNotRespondingReason indicates that the api server cannot be reached.
	KubeAPIServerNotRespondingReason = "KubeAPIServerNotResponding"
	// TargetClusterCreateFailedReason indicates that the target cluster could not be created.
	TargetClusterCreateFailedReason = "TargetClusterCreateFailed"
	// TargetClusterControlPlaneNotReadyReason indicates that the target cluster's control plane is not ready yet.
	TargetClusterControlPlaneNotReadyReason = "TargetClusterControlPlaneNotReady"
	// ControlPlaneEndpointSetCondition indicates that the control plane is set.
	ControlPlaneEndpointSetCondition = "ControlPlaneEndpointSet"
)

const (
	// TargetClusterSecretReadyCondition reports on whether the libvirt secret in the target cluster is ready.
	TargetClusterSecretReadyCondition clusterv1.ConditionType = "TargetClusterSecretReady"
	// TargetSecretSyncFailedReason indicates that the target secret could not be synced.
	TargetSecretSyncFailedReason = "TargetSecretSyncFailed"
	// ControlPlaneEndpointNotSetReason indicates that the control plane endpoint is not set.
	ControlPlaneEndpointNotSetReason = "ControlPlaneEndpointNotSet"
)

const (
	// LibvirtAPIReachableCondition reports whether the Libvirt APIs are reachable.
	LibvirtAPIReachableCondition clusterv1.ConditionType = "LibvirtAPIReachable"
	// RateLimitExceededReason indicates that a rate limit has been exceeded.
	RateLimitExceededReason = "RateLimitExceeded"
)

const (
	// DomainCreateSucceededCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	DomainCreateSucceededCondition clusterv1.ConditionType = "DomainCreateSucceeded"
	// DomainCreateFailedReason indicates that Domain could not get created.
	DomainCreateFailedReason = "DomainrCreateFailedReason"
)
