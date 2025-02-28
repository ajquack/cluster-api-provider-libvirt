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

// Package scope defines cluster and machine scope as well as a repository for the Libvirt API.
package scope

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2/textlogger"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
)

// LibvirtMachineTemplateScopeParams defines the input parameters used to create a new scope.
type LibvirtMachineTemplateScopeParams struct {
	Client                 client.Client
	Logger                 *logr.Logger
	LibvirtClient          libvirtclient.Client
	LibvirtMachineTemplate *infrav1alpha1.LibvirtMachineTemplate
}

// NewLibvirtMachineTemplateScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewLibvirtMachineTemplateScope(params LibvirtMachineTemplateScopeParams) (*LibvirtMachineTemplateScope, error) {
	if params.LibvirtClient == nil {
		return nil, errors.New("failed to generate new scope from nil LibvirtClient")
	}

	if params.Logger == nil {
		logger := textlogger.NewLogger(textlogger.NewConfig())
		params.Logger = &logger
	}

	helper, err := patch.NewHelper(params.LibvirtMachineTemplate, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &LibvirtMachineTemplateScope{
		Logger:                 params.Logger,
		Client:                 params.Client,
		LibvirtMachineTemplate: params.LibvirtMachineTemplate,
		LibvirtClient:          params.LibvirtClient,
		patchHelper:            helper,
	}, nil
}

// LibvirtMachineTemplateScope defines the basic context for an actuator to operate upon.
type LibvirtMachineTemplateScope struct {
	*logr.Logger
	Client        client.Client
	patchHelper   *patch.Helper
	LibvirtClient libvirtclient.Client

	LibvirtMachineTemplate *infrav1alpha1.LibvirtMachineTemplate
}

// Name returns the LibvirtMachineTemplate name.
func (s *LibvirtMachineTemplateScope) Name() string {
	return s.LibvirtMachineTemplate.Name
}

// Namespace returns the namespace name.
func (s *LibvirtMachineTemplateScope) Namespace() string {
	return s.LibvirtMachineTemplate.Namespace
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *LibvirtMachineTemplateScope) Close(ctx context.Context) error {
	conditions.SetSummary(s.LibvirtMachineTemplate)
	return s.patchHelper.Patch(ctx, s.LibvirtMachineTemplate)
}

// PatchObject persists the machine spec and status.
func (s *LibvirtMachineTemplateScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.LibvirtMachineTemplate)
}
