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

package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
	util "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/utils"
)

// LibvirtRemediationScopeParams defines the input parameters used to create a new Scope.
type LibvirtRemediationScopeParams struct {
	Logger             logr.Logger
	Client             client.Client
	LibvirtClient      libvirtclient.Client
	Machine            *clusterv1.Machine
	LibvirtMachine     *infrav1alpha1.LibvirtMachine
	LibvirtCluster     *infrav1alpha1.LibvirtCluster
	LibvirtRemediation *infrav1alpha1.LibvirtRemediation
}

// NewLibvirtRemediationScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewLibvirtRemediationScope(params LibvirtRemediationScopeParams) (*LibvirtRemediationScope, error) {
	if params.LibvirtRemediation == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtRemediation")
	}
	if params.Client == nil {
		return nil, errors.New("failed to generate new scope from nil client")
	}
	if params.LibvirtClient == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtClient")
	}
	if params.Machine == nil {
		return nil, errors.New("failed to generate new scope from nil Machine")
	}
	if params.LibvirtMachine == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtMachine")
	}

	emptyLogger := logr.Logger{}
	if params.Logger == emptyLogger {
		return nil, errors.New("failed to generate new scope from nil Logger")
	}

	patchHelper, err := patch.NewHelper(params.LibvirtRemediation, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	machinePatchHelper, err := patch.NewHelper(params.Machine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init machine patch helper: %w", err)
	}

	return &LibvirtRemediationScope{
		Logger:             params.Logger,
		Client:             params.Client,
		LibvirtClient:      params.LibvirtClient,
		patchHelper:        patchHelper,
		machinePatchHelper: machinePatchHelper,
		Machine:            params.Machine,
		LibvirtMachine:     params.LibvirtMachine,
		LibvirtRemediation: params.LibvirtRemediation,
	}, nil
}

// LibvirtRemediationScope defines the basic context for an actuator to operate upon.
type LibvirtRemediationScope struct {
	logr.Logger
	Client             client.Client
	patchHelper        *patch.Helper
	machinePatchHelper *patch.Helper
	LibvirtClient      libvirtclient.Client
	Machine            *clusterv1.Machine
	LibvirtMachine     *infrav1alpha1.LibvirtMachine
	LibvirtRemediation *infrav1alpha1.LibvirtRemediation
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *LibvirtRemediationScope) Close(ctx context.Context, opts ...patch.Option) error {
	conditions.SetSummary(m.LibvirtRemediation)
	return m.patchHelper.Patch(ctx, m.LibvirtRemediation, opts...)
}

// Name returns the LibvirtMachine name.
func (m *LibvirtRemediationScope) Name() string {
	return m.LibvirtRemediation.Name
}

// Namespace returns the namespace name.
func (m *LibvirtRemediationScope) Namespace() string {
	return m.LibvirtRemediation.Namespace
}

// ServerIDFromProviderID returns the namespace name.
func (m *LibvirtRemediationScope) DomainIDFromProviderID() (string, error) {
	return util.DomainIDFromProviderID(m.LibvirtMachine.Spec.ProviderID)
}

// PatchObject persists the remediation spec and status.
func (m *LibvirtRemediationScope) PatchObject(ctx context.Context, opts ...patch.Option) error {
	return m.patchHelper.Patch(ctx, m.LibvirtRemediation, opts...)
}

// PatchMachine persists the machine spec and status.
func (m *LibvirtRemediationScope) PatchMachine(ctx context.Context, opts ...patch.Option) error {
	return m.machinePatchHelper.Patch(ctx, m.Machine, opts...)
}
