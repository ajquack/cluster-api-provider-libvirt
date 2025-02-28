package virtualMachine

import (
	"context"
	"errors"
	"fmt"
	"time"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
	utils "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/utils"
	"github.com/google/uuid"
	libvirt "github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DomainOffTimeout = 10 * time.Minute
)

var (
	errDomainCreateNotPossible = fmt.Errorf("domain create not possible - need action")
)

type Service struct {
	scope *scope.MachineScope
}

func NewService(scope *scope.MachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	// waiting for bootstrap data to be ready
	if !s.scope.IsBootstrapDataReady() {
		conditions.MarkFalse(
			s.scope.LibvirtMachine,
			infrav1alpha1.BootstrapReadyCondition,
			infrav1alpha1.BootstrapNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	conditions.MarkTrue(s.scope.LibvirtMachine, infrav1alpha1.BootstrapReadyCondition)

	// try to find an existing domain
	domain, err := s.findDomain(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get domain: %w", err)
	}

	// if no domain is found we have to create one
	if domain == nil {
		domain, err = s.CreateDomain(ctx)
		if err != nil {
			if errors.Is(err, errDomainCreateNotPossible) {
				return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			return reconcile.Result{}, fmt.Errorf("failed to create domain: %w", err)
		}
	}

	uuidStr, err := domain.GetUUIDString()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get UUID string: %w", err)
	}
	uuid, err := uuid.Parse(uuidStr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to parse UUID: %w", err)
	}
	s.scope.SetProviderID(uuid)

	// update HCloudMachineStatus
	c := s.scope.LibvirtMachine.Status.Conditions.DeepCopy()
	s.scope.LibvirtMachine.Status = statusFromLibvirtDomain(domain)
	s.scope.LibvirtMachine.Status.Conditions = c

	// analyze status of domain
	state, _, err := domain.GetState()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get domain state: %w", err)
	}

	switch state {
	case libvirt.DOMAIN_SHUTOFF:
		return s.handleDomainStatusOff(ctx, domain)
	case libvirt.DOMAIN_BLOCKED:
		// Requeue here so that domain does not switch back and forth between off and starting.
		// If we don't return here, the condition DomainAvailable would get marked as true in this
		// case. However, if the domain is stuck and does not power on, we should not mark the
		// condition domainAvailable as true to be able to remediate the domain after a timeout.
		conditions.MarkFalse(
			s.scope.LibvirtMachine,
			infrav1alpha1.DomainAvailableCondition,
			infrav1alpha1.DomainStartingReason,
			clusterv1.ConditionSeverityInfo,
			"domain is starting",
		)
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	case libvirt.DOMAIN_RUNNING: // do nothing
	default:
		// some temporary status
		s.scope.SetReady(false)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// nothing to do any more for worker nodes
	if !s.scope.IsControlPlane() {
		conditions.MarkTrue(s.scope.LibvirtMachine, infrav1alpha1.DomainAvailableCondition)
		s.scope.SetReady(true)
		return res, nil
	}

	s.scope.SetReady(true)
	conditions.MarkTrue(s.scope.LibvirtMachine, infrav1alpha1.DomainAvailableCondition)

	return res, nil
}

func (s *Service) CreateDomain(ctx context.Context) (*libvirt.Domain, error) {
	// get userData
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		record.Warnf(
			s.scope.LibvirtMachine,
			"FailedGetBootstrapData",
			err.Error(),
		)
		return nil, fmt.Errorf("failed to get raw bootstrap data: %s", err)
	}

	// Create cloud-init disk
	cloudInitDiskPath := s.scope.LibvirtClient.CreateCloudInitDisk(ctx, userData, "")
	if cloudInitDiskPath != nil {
		record.Warnf(
			s.scope.LibvirtMachine,
			"FailedCreateCloudInitDisk",
			cloudInitDiskPath.Error(),
		)
		return nil, fmt.Errorf("failed to create cloud-init disk: %s", err)
	}

	opts := libvirtxml.Domain{
		Type: "kvm",
		Name: s.scope.Name(),
		Devices: &libvirtxml.DomainDeviceList{
			Disks: []libvirtxml.DomainDisk{
				{
					Device: "disk",
					Driver: &libvirtxml.DomainDiskDriver{
						Name: "qemu",
						Type: "qcow2",
					},
					Source: &libvirtxml.DomainDiskSource{
						File: &libvirtxml.DomainDiskSourceFile{
							File: s.scope.LibvirtMachine.Spec.Image,
						},
					},
					Target: &libvirtxml.DomainDiskTarget{
						Dev: "vda",
						Bus: "virtio",
					},
				},
			},
			Interfaces: []libvirtxml.DomainInterface{
				{
					Source: &libvirtxml.DomainInterfaceSource{
						Network: &libvirtxml.DomainInterfaceSourceNetwork{
							Network: s.scope.LibvirtMachine.Spec.Network,
						},
					},
					Model: &libvirtxml.DomainInterfaceModel{
						Type: "virtio",
					},
				},
			},
		},
		Memory: &libvirtxml.DomainMemory{
			Value: uint(s.scope.LibvirtMachine.Spec.Memory),
			Unit:  "MiB",
		},
		VCPU: &libvirtxml.DomainVCPU{
			Value: uint(s.scope.LibvirtMachine.Spec.CPU),
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Arch:    "x86_64",
				Machine: "pc",
				Type:    "hvm",
			},
		},
	}

	domainXML, err := opts.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal domain XML: %w", err)
	}

	// Create the server
	server, err := s.scope.LibvirtClient.CreateDomain(ctx, domainXML)

	if err != nil {
		if utils.HandleRateLimitExceeded(s.scope.LibvirtMachine, err, "CreateDomain") {
			// RateLimit was reached. Condition and Event got already created.
			return nil, fmt.Errorf("failed to create Libvirt domain %s: %w", s.scope.LibvirtMachine.Name, err)
		}
		// No condition was set yet. Set a general condition to false.
		conditions.MarkFalse(
			s.scope.LibvirtMachine,
			infrav1alpha1.DomainCreateSucceededCondition,
			infrav1alpha1.DomainCreateFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		record.Warnf(s.scope.LibvirtMachine,
			"FailedCreateHCloudServer",
			"Failed to create HCloud server %s: %s",
			s.scope.Name(),
			err,
		)
		errMsg := fmt.Sprintf("failed to create Libvirt domain %s", s.scope.LibvirtMachine.Name)
		return nil, handleRateLimit(s.scope.LibvirtMachine, err, "CreateDomain", errMsg)
	}

	conditions.MarkTrue(s.scope.LibvirtMachine, infrav1alpha1.DomainCreateSucceededCondition)
	serverName, nameErr := server.GetName()
	if nameErr != nil {
		return nil, fmt.Errorf("failed to get server name: %w", nameErr)
	}
	serverUUID, uuidErr := server.GetUUID()
	if uuidErr != nil {
		return nil, fmt.Errorf("failed to get server UUID: %w", uuidErr)
	}
	record.Eventf(s.scope.LibvirtMachine, "SuccessfulCreate", "Created new server %s with ID %d", serverName, serverUUID)
	return server, nil
}

func (s *Service) DeleteDomain(ctx context.Context) (res reconcile.Result, err error) {
	domain, err := s.findDomain(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get virtual machine: %w", err)
	}

	// Check if virtual machine exists before trying to delete it
	if domain == nil {
		msg := fmt.Sprintf("Unable to delete virtual machine. Could not find domain for %s", s.scope.Name())
		s.scope.V(1).Info(msg)
		record.Warnf(s.scope.LibvirtMachine, "NoVirtualMachineFound", msg)
		return res, nil
	}

	// Shutdown the virtual machine and delete it afterwards
	state, _, err := domain.GetState()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get domain state: %w", err)
	}

	switch state {
	case libvirt.DOMAIN_RUNNING:
		return s.handleDeleteDomainStatusRunning(ctx, domain)
	case libvirt.DOMAIN_SHUTOFF:
		return s.handleDeleteDomainStatusOff(ctx, domain)
	}

	return reconcile.Result{}, nil
}

func (s *Service) findDomain(ctx context.Context) (*libvirt.Domain, error) {
	var domain *libvirt.Domain
	domainID, err := s.scope.DomainIDFromProviderID()
	if err == nil {
		domain, err = s.scope.LibvirtClient.GetDomain(ctx, domainID)
		if err != nil {
			return nil, fmt.Errorf("failed to get domain: %w", err)
		}

		if domain != nil {
			return domain, nil
		}
	}
	return nil, nil
}

func statusFromLibvirtDomain(server *libvirt.Domain) infrav1alpha1.LibvirtMachineStatus {
	// set instance state
	state, _, err := server.GetState()
	if err != nil {
		return infrav1alpha1.LibvirtMachineStatus{}
	}

	instanceState := utils.GetDomainStateString(state)

	return infrav1alpha1.LibvirtMachineStatus{
		InstanceState: &instanceState,
	}
}

func (s *Service) handleDomainStatusOff(ctx context.Context, domain *libvirt.Domain) (res reconcile.Result, err error) {
	// Check if domain is in DomainStatusOff and turn it on. This is to avoid a bug of Hetzner where
	// sometimes machines are created and not turned on

	domainAvailableCondition := conditions.Get(s.scope.LibvirtMachine, infrav1alpha1.DomainAvailableCondition)
	if domainAvailableCondition != nil &&
		domainAvailableCondition.Status == corev1.ConditionFalse &&
		domainAvailableCondition.Reason == infrav1alpha1.DomainOffReason {
		s.scope.Info("Trigger power on again")
		if time.Now().Before(domainAvailableCondition.LastTransitionTime.Time.Add(DomainOffTimeout)) {
			// Not yet timed out, try again to power on
			if err := s.scope.LibvirtClient.StartDomain(ctx, domain); err != nil {
				state, _, err := domain.GetState()
				if err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to get domain state: %w", err)
				}
				if state != libvirt.DOMAIN_RUNNING {
					// if domain is locked, we just retry again
					return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
				}
				return reconcile.Result{}, handleRateLimit(s.scope.LibvirtMachine, err, "PowerOndomain", "failed to power on domain")
			}
		} else {
			// Timed out. Set failure reason
			s.scope.SetError("reached timeout of waiting for machines that are switched off", capierrors.CreateMachineError)
			return res, nil
		}
	} else {
		// No condition set yet. Try to power domain on.
		if err := s.scope.LibvirtClient.StartDomain(ctx, domain); err != nil {
			state, _, err := domain.GetState()
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to get domain state: %w", err)
			}
			if state != libvirt.DOMAIN_RUNNING {
				// if domain is locked, we just retry again
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return reconcile.Result{}, handleRateLimit(s.scope.LibvirtMachine, err, "PowerOndomain", "failed to power on domain")
		}
		conditions.MarkFalse(
			s.scope.LibvirtMachine,
			infrav1alpha1.DomainAvailableCondition,
			infrav1alpha1.DomainOffReason,
			clusterv1.ConditionSeverityInfo,
			"domain is switched off",
		)
	}

	// Try again in 30 sec.
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

func (s *Service) handleDeleteDomainStatusRunning(ctx context.Context, domain *libvirt.Domain) (res reconcile.Result, err error) {
	if s.scope.HasServerAvailableCondition() {
		if err := s.scope.LibvirtClient.ShutdownDomain(ctx, domain); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to shutdown domain: %w", err)
		}

		conditions.MarkFalse(s.scope.LibvirtMachine,
			infrav1alpha1.DomainAvailableCondition,
			infrav1alpha1.DomainTerminatingReason,
			clusterv1.ConditionSeverityInfo,
			"Domain has been shut down",
		)

		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if err := s.scope.LibvirtClient.DestroyDomain(ctx, domain); err != nil {
		record.Warnf(s.scope.LibvirtMachine, "FailedDeleteDomain", "Failed to delete domain: %v", err)
		return reconcile.Result{}, fmt.Errorf("failed to delete domain: %w", err)
	}
	record.Eventf(s.scope.LibvirtMachine, "LibvirtMachineDeleted", "Deleted domain %s", s.scope.Name())
	return res, nil
}

func (s *Service) handleDeleteDomainStatusOff(ctx context.Context, domain *libvirt.Domain) (res reconcile.Result, err error) {
	// Domain is already shut down, delete it
	if err := s.scope.LibvirtClient.DestroyDomain(ctx, domain); err != nil {
		record.Warnf(s.scope.LibvirtMachine, "FailedDeleteDomain", "Failed to delete domain: %v", err)
		return reconcile.Result{}, fmt.Errorf("failed to delete domain: %w", err)
	}
	record.Eventf(s.scope.LibvirtMachine, "LibvirtMachineDeleted", "Deleted domain %s", s.scope.Name())
	return res, nil
}

// implements setting rate limit on libvirtMachine.
func handleRateLimit(hm *infrav1alpha1.LibvirtMachine, err error, functionName string, errMsg string) error {
	// returns error if not a rate limit exceeded error
	if _, ok := err.(libvirt.Error); !ok {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	// does not return error if machine is running and does not have a deletion timestamp
	if hm.Status.Ready && hm.DeletionTimestamp.IsZero() {
		return nil
	}

	// check for a rate limit exceeded error if the machine is not running or if machine has a deletion timestamp
	utils.HandleRateLimitExceeded(hm, err, functionName)
	return fmt.Errorf("%s: %w", errMsg, err)
}
