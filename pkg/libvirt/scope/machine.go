package scope

import (
	"context"
	"errors"
	"fmt"
	"time"

	uuid "github.com/google/uuid"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	secrets "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/utils"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// MachineScopeParams defines the input parameters used to create a new Scope.
type MachineScopeParams struct {
	ClusterScopeParams
	Machine        *clusterv1.Machine
	LibvirtMachine *infrav1alpha1.LibvirtMachine
}

const maxShutDownTime = 2 * time.Minute

var (
	// ErrBootstrapDataNotReady return an error if no bootstrap data is ready.
	ErrBootstrapDataNotReady = errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	// ErrEmptyProviderID indicates an empty providerID.
	ErrEmptyProviderID = fmt.Errorf("providerID is empty")
	// ErrInvalidProviderID indicates an invalid providerID.
	ErrInvalidProviderID = fmt.Errorf("providerID is invalid")
	// ErrInvalidServerID indicates an invalid serverID.
	ErrInvalidServerID = fmt.Errorf("serverID is invalid")
)

type MachineScope struct {
	ClusterScope
	Machine        *clusterv1.Machine
	LibvirtMachine *infrav1alpha1.LibvirtMachine
}

func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.LibvirtMachine == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtMachine")
	}

	cs, err := NewClusterScope(params.ClusterScopeParams)
	if err != nil {
		return nil, fmt.Errorf("failed create new cluster scope: %w", err)
	}

	cs.patchHelper, err = patch.NewHelper(params.LibvirtMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &MachineScope{
		ClusterScope:   *cs,
		Machine:        params.Machine,
		LibvirtMachine: params.LibvirtMachine,
	}, nil
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close(ctx context.Context) error {
	conditions.SetSummary(m.LibvirtMachine)
	return m.patchHelper.Patch(ctx, m.LibvirtMachine)
}

// Name returns the LibvirtMachine name.
func (m *MachineScope) Name() string {
	return m.LibvirtMachine.Name
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.LibvirtMachine.Namespace
}

// PatchObject persists the machine spec and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.LibvirtMachine)
}

// SetError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (m *MachineScope) SetError(message string, reason capierrors.MachineStatusError) {
	m.LibvirtMachine.Status.FailureMessage = &message
	m.LibvirtMachine.Status.FailureReason = &reason
}

// SetProviderID sets the providerID field on the machine.
func (m *MachineScope) SetProviderID(serverID uuid.UUID) {
	providerID := fmt.Sprintf("libvirt://%d", serverID)
	m.LibvirtMachine.Spec.ProviderID = &providerID
}

func (m *MachineScope) DomainIDFromProviderID() (string, error) {
	if m.LibvirtMachine.Spec.ProviderID == nil || m.LibvirtMachine.Spec.ProviderID != nil && *m.LibvirtMachine.Spec.ProviderID == "" {
		return "", ErrEmptyProviderID
	}
	return utils.DomainIDFromProviderID(m.Machine.Spec.ProviderID)
}

// SetReady sets the ready field on the machine.
func (m *MachineScope) SetReady(ready bool) {
	m.LibvirtMachine.Status.Ready = ready
}

// HasServerAvailableCondition checks whether ServerAvailable condition is set on true.
func (m *MachineScope) HasServerAvailableCondition() bool {
	return conditions.IsTrue(m.LibvirtMachine, infrav1alpha1.DomainAvailableCondition)
}

// HasServerTerminatedCondition checks the whether ServerAvailable condition is false with reason "terminated".
func (m *MachineScope) HasServerTerminatedCondition() bool {
	return conditions.IsFalse(m.LibvirtMachine, infrav1alpha1.DomainAvailableCondition) &&
		conditions.GetReason(m.LibvirtMachine, infrav1alpha1.DomainAvailableCondition) == infrav1alpha1.DomainTerminatingReason
}

// HasShutdownTimedOut checks the whether the HCloud server is terminated.
func (m *MachineScope) HasShutdownTimedOut() bool {
	return time.Now().After(conditions.GetLastTransitionTime(m.LibvirtMachine, infrav1alpha1.DomainAvailableCondition).Time.Add(maxShutDownTime))
}

// IsBootstrapDataReady checks the readiness of a capi machine's bootstrap data.
func (m *MachineScope) IsBootstrapDataReady() bool {
	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, ErrBootstrapDataNotReady
	}

	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	secretManager := secrets.NewSecretManager(m.Logger, m.Client, m.APIReader)
	secret, err := secretManager.AcquireSecret(ctx, key, m.LibvirtMachine, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire secret: %w", err)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
