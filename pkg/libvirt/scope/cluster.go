package scope

import (
	"context"
	"errors"
	"fmt"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
	secrets "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
)

type ClusterScopeParams struct {
	Client         client.Client
	APIReader      client.Reader
	Logger         logr.Logger
	Cluster        *clusterv1.Cluster
	LibvirtSecret  *corev1.Secret
	LibvirtClient  libvirtclient.Client
	LibvirtCluster *infrav1alpha1.LibvirtCluster
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client         client.Client
	APIReader      client.Reader
	patchHelper    *patch.Helper
	libvirtSecret  *corev1.Secret
	LibvirtClient  libvirtclient.Client
	Cluster        *clusterv1.Cluster
	LibvirtCluster *infrav1alpha1.LibvirtCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.LibvirtCluster == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtCluster")
	}
	if params.LibvirtClient == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtClient")
	}
	if params.APIReader == nil {
		return nil, errors.New("failed to generate new scope from nil APIReader")
	}
	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	helper, err := patch.NewHelper(params.LibvirtCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ClusterScope{
		Logger:         params.Logger,
		Client:         params.Client,
		APIReader:      params.APIReader,
		Cluster:        params.Cluster,
		LibvirtCluster: params.LibvirtCluster,
		LibvirtClient:  params.LibvirtClient,
		patchHelper:    helper,
		libvirtSecret:  params.LibvirtSecret,
	}, nil
}

// Name returns the LibvirtCluster name.
func (s *ClusterScope) Name() string {
	return s.LibvirtCluster.Name
}

// Namespace returns the namespace name.
func (s *ClusterScope) Namespace() string {
	return s.LibvirtCluster.Namespace
}

// HetznerSecret returns the hetzner secret.
func (s *ClusterScope) LibvirtSecret() *corev1.Secret {
	return s.libvirtSecret
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	conditions.SetSummary(s.LibvirtCluster)
	return s.patchHelper.Patch(ctx, s.LibvirtCluster)
}

// PatchObject persists the machine spec and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.LibvirtCluster)
}

// ControlPlaneAPIEndpointPort returns the Port of the Kube-api server.
func (s *ClusterScope) ControlPlaneAPIEndpointPort() int32 {
	return int32(s.LibvirtCluster.Spec.ControlPlaneEndpoint.Port) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
}

// ClientConfig return a kubernetes client config for the cluster context.
func (s *ClusterScope) ClientConfig(ctx context.Context) (clientcmd.ClientConfig, error) {
	cluster := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s", s.Cluster.Name, secret.Kubeconfig),
		Namespace: s.Cluster.Namespace,
	}

	secretManager := secrets.NewSecretManager(s.Logger, s.Client, s.APIReader)
	kubeconfigSecret, err := secretManager.AcquireSecret(ctx, cluster, s.LibvirtCluster, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}
	kubeconfigBytes, ok := kubeconfigSecret.Data[secret.KubeconfigDataName]
	if !ok {
		return nil, fmt.Errorf("missing key %q in secret data", secret.KubeconfigDataName)
	}
	return clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
}

// ClientConfigWithAPIEndpoint returns a client config.
func (s *ClusterScope) ClientConfigWithAPIEndpoint(ctx context.Context, endpoint clusterv1.APIEndpoint) (clientcmd.ClientConfig, error) {
	c, err := s.ClientConfig(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := c.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("error retrieving rawConfig from clientConfig: %w", err)
	}
	// update cluster endpint in config
	for key := range raw.Clusters {
		raw.Clusters[key].Server = fmt.Sprintf("https://%s:%d", endpoint.Host, endpoint.Port)
	}

	return clientcmd.NewDefaultClientConfig(raw, &clientcmd.ConfigOverrides{}), nil
}

// ListMachines returns LibvirtMachines.
func (s *ClusterScope) ListMachines(ctx context.Context) ([]*clusterv1.Machine, []*infrav1alpha1.LibvirtMachine, error) {
	// get and index Machines by LibvirtMachine name
	var machineListRaw clusterv1.MachineList
	machineByLibvirtMachineName := make(map[string]*clusterv1.Machine)
	if err := s.Client.List(ctx, &machineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}
	expectedGK := infrav1alpha1.GroupVersion.WithKind("LibvirtMachine").GroupKind()
	for pos := range machineListRaw.Items {
		m := &machineListRaw.Items[pos]
		actualGK := m.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if m.Spec.ClusterName != s.Cluster.Name ||
			actualGK.String() != expectedGK.String() {
			continue
		}
		machineByLibvirtMachineName[m.Spec.InfrastructureRef.Name] = m
	}

	// match LibvirtMachines to Machines
	var LibvirtMachineListRaw infrav1alpha1.LibvirtMachineList
	if err := s.Client.List(ctx, &LibvirtMachineListRaw, client.InNamespace(s.Namespace())); err != nil {
		return nil, nil, err
	}

	machineList := make([]*clusterv1.Machine, 0, len(LibvirtMachineListRaw.Items))
	LibvirtMachineList := make([]*infrav1alpha1.LibvirtMachine, 0, len(LibvirtMachineListRaw.Items))

	for pos := range LibvirtMachineListRaw.Items {
		hm := &LibvirtMachineListRaw.Items[pos]
		m, ok := machineByLibvirtMachineName[hm.Name]
		if !ok {
			continue
		}

		machineList = append(machineList, m)
		LibvirtMachineList = append(LibvirtMachineList, hm)
	}

	return machineList, LibvirtMachineList, nil
}

// IsControlPlaneReady returns nil if the control plane is ready.
func IsControlPlaneReady(ctx context.Context, c clientcmd.ClientConfig) error {
	restConfig, err := c.ClientConfig()
	if err != nil {
		return err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	_, err = clientSet.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(ctx)
	return err
}
