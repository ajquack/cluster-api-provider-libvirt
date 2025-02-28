package machinetemplate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
)

// Service defines struct with LibvirtMachineTemplate scope to reconcile Libvirt machine templates.
type Service struct {
	scope *scope.LibvirtMachineTemplateScope
}

// NewService outs a new service with LibvirtMachineTemplate scope.
func NewService(scope *scope.LibvirtMachineTemplateScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of LibvirtMachinesTemplates.
func (s *Service) Reconcile(ctx context.Context) error {
	if s.scope.LibvirtMachineTemplate.Status.Capacity == nil {
		capacity, err := s.getCapacity()
		if err != nil {
			return fmt.Errorf("failed to get capacity: %w", err)
		}

		s.scope.LibvirtMachineTemplate.Status.Capacity = capacity
	}
	return nil
}

func (s *Service) getCapacity() (corev1.ResourceList, error) {
	capacity := make(corev1.ResourceList)
	cpu, err := GetCPUQuantityFromInt(s.scope.LibvirtMachineTemplate.Spec.Template.Spec.CPU)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quantity. CPU %v: %w", s.scope.LibvirtMachineTemplate.Spec.Template.Spec.CPU, err)
	}
	capacity[corev1.ResourceCPU] = cpu
	memory, err := GetMemoryQuantityFromInt(s.scope.LibvirtMachineTemplate.Spec.Template.Spec.Memory)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quantity. Memory %v: %w", s.scope.LibvirtMachineTemplate.Spec.Template.Spec.Memory, err)
	}
	capacity[corev1.ResourceMemory] = memory

	return capacity, nil
}
