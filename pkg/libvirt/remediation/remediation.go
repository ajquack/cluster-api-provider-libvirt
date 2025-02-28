/*
Copyright 2022 The Kubernetes Authors.

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

// Package remediation implements functions to manage the lifecycle of libvirt remediation.
package remediation

import (
	"context"
	"fmt"
	"time"

	libvirt "github.com/libvirt/libvirt-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
	utils "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/utils"
)

// Service defines struct with machine scope to reconcile LibvirtRemediation.
type Service struct {
	scope *scope.LibvirtRemediationScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.LibvirtRemediationScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of LibvirtRemediation.
func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	domain, err := s.findDomain(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to find the domain of unhealthy machine: %w", err)
	}

	// stop remediation if domain does not exist
	if domain == nil {
		s.scope.LibvirtRemediation.Status.Phase = infrav1alpha1.PhaseDeleting

		if err := s.setOwnerRemediatedCondition(ctx); err != nil {
			record.Warn(s.scope.LibvirtRemediation, "FailedSettingConditionOnMachine", err.Error())
			return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
		}
		record.Warn(s.scope.LibvirtRemediation, "ExitRemediation", "exit remediation because bare metal domain does not exist")
		return res, nil
	}

	remediationType := s.scope.LibvirtRemediation.Spec.Strategy.Type

	if remediationType != infrav1alpha1.RemediationTypeReboot {
		s.scope.Info("unsupported remediation strategy")
		record.Warnf(s.scope.LibvirtRemediation, "UnsupportedRemdiationStrategy", "remediation strategy %q is unsupported", remediationType)
		return res, nil
	}

	// If no phase set, default to running
	if s.scope.LibvirtRemediation.Status.Phase == "" {
		s.scope.LibvirtRemediation.Status.Phase = infrav1alpha1.PhaseRunning
	}

	switch s.scope.LibvirtRemediation.Status.Phase {
	case infrav1alpha1.PhaseRunning:
		return s.handlePhaseRunning(ctx, domain)
	case infrav1alpha1.PhaseWaiting:
		return s.handlePhaseWaiting(ctx)
	}

	return res, nil
}

func (s *Service) handlePhaseRunning(ctx context.Context, domain *libvirt.Domain) (res reconcile.Result, err error) {
	now := metav1.Now()

	domainName, err := domain.GetName()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get domain name: %w", err)
	}
	domainUUID, err := domain.GetUUIDString()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get domain UUID: %w", err)
	}

	// if domain has never been remediated, then do that now
	if s.scope.LibvirtRemediation.Status.LastRemediated == nil {
		if err := s.scope.LibvirtClient.RebootDomain(ctx, domain); err != nil {
			utils.HandleRateLimitExceeded(s.scope.LibvirtMachine, err, "RebootDomain")
			record.Warn(s.scope.LibvirtRemediation, "FailedRebootDomain", err.Error())
			return reconcile.Result{}, fmt.Errorf("failed to reboot domain %s with ID %d: %w", domainName, domainUUID, err)
		}
		record.Event(s.scope.LibvirtRemediation, "DomainRebooted", "domain has been rebooted")

		s.scope.LibvirtRemediation.Status.LastRemediated = &now
		s.scope.LibvirtRemediation.Status.RetryCount++
	}

	retryLimit := s.scope.LibvirtRemediation.Spec.Strategy.RetryLimit
	retryCount := s.scope.LibvirtRemediation.Status.RetryCount

	// check whether retry limit has been reached
	if retryLimit == 0 || retryCount >= retryLimit {
		s.scope.LibvirtRemediation.Status.Phase = infrav1alpha1.PhaseWaiting
	}

	// check when next remediation should be scheduled
	nextRemediation := s.timeUntilNextRemediation(now.Time)

	if nextRemediation > 0 {
		// Not yet time to remediate, requeue
		return reconcile.Result{RequeueAfter: nextRemediation}, nil
	}

	// remediate now
	if err := s.scope.LibvirtClient.RebootDomain(ctx, domain); err != nil {
		utils.HandleRateLimitExceeded(s.scope.LibvirtMachine, err, "RebootDomain")
		record.Warn(s.scope.LibvirtRemediation, "FailedRebootDomain", err.Error())
		return reconcile.Result{}, fmt.Errorf("failed to reboot domain %s with ID %d: %w", domainName, domainUUID, err)
	}
	record.Event(s.scope.LibvirtRemediation, "DomainRebooted", "Domain has been rebooted")

	s.scope.LibvirtRemediation.Status.LastRemediated = &now
	s.scope.LibvirtRemediation.Status.RetryCount++

	return res, nil
}

func (s *Service) handlePhaseWaiting(ctx context.Context) (res reconcile.Result, err error) {
	nextCheck := s.timeUntilNextRemediation(time.Now())

	if nextCheck > 0 {
		// Not yet time to stop remediation, requeue
		return reconcile.Result{RequeueAfter: nextCheck}, nil
	}

	// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
	// moves control to CAPI machine controller. The owning controller will do
	// preflight checks and handles the Machine deletion

	s.scope.LibvirtRemediation.Status.Phase = infrav1alpha1.PhaseDeleting

	if err := s.setOwnerRemediatedCondition(ctx); err != nil {
		record.Warn(s.scope.LibvirtRemediation, "FailedSettingConditionOnMachine", err.Error())
		return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
	}
	record.Event(s.scope.LibvirtRemediation, "SetOwnerRemediatedCondition", "exit remediation because because retryLimit is reached and reboot timed out")

	return res, nil
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

// setOwnerRemediatedCondition sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck.
func (s *Service) setOwnerRemediatedCondition(ctx context.Context) error {
	conditions.MarkFalse(
		s.scope.Machine,
		clusterv1.MachineOwnerRemediatedCondition,
		clusterv1.WaitingForRemediationReason,
		clusterv1.ConditionSeverityWarning,
		"",
	)
	if err := s.scope.PatchMachine(ctx); err != nil {
		return fmt.Errorf("failed to patch machine: %w", err)
	}
	return nil
}

// timeUntilNextRemediation checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (s *Service) timeUntilNextRemediation(now time.Time) time.Duration {
	timeout := s.scope.LibvirtRemediation.Spec.Strategy.Timeout.Duration
	// status is not updated yet
	if s.scope.LibvirtRemediation.Status.LastRemediated == nil {
		return timeout
	}

	if s.scope.LibvirtRemediation.Status.LastRemediated.Add(timeout).Before(now) {
		return time.Duration(0)
	}

	lastRemediated := now.Sub(s.scope.LibvirtRemediation.Status.LastRemediated.Time)
	nextRemediation := timeout - lastRemediated + time.Second
	return nextRemediation
}
