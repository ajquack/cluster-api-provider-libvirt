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

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
	machinetemplate "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/machineTemplate"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
	secrets "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
)

// LibvirtMachineTemplateReconciler reconciles a LibvirtMachineTemplate object.
type LibvirtMachineTemplateReconciler struct {
	client.Client
	RateLimitWaitTime    time.Duration
	APIReader            client.Reader
	LibvirtclientFactory libvirtclient.Factory
	WatchFilterValue     string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=Libvirtmachinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=Libvirtmachinetemplates/status,verbs=get;update;patch

// Reconcile manages the lifecycle of an LibvirtMachineTemplate object.
func (r *LibvirtMachineTemplateReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	machineTemplate := &infrav1alpha1.LibvirtMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, machineTemplate); err != nil {
		log.Error(err, "unable to fetch LibvirtMachineTemplate")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("LibvirtMachineTemplate", klog.KObj(machineTemplate))

	patchHelper, err := patch.NewHelper(machineTemplate, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get patch helper: %w", err)
	}

	defer func() {
		if err := patchHelper.Patch(ctx, machineTemplate); err != nil {
			log.Error(err, "failed to patch LibvirtMachineTemplate")
		}
	}()

	// Check whether owner is a ClusterClass. In that case there is nothing to do.
	if hasOwnerClusterClass(machineTemplate.ObjectMeta) {
		machineTemplate.Status.OwnerType = "ClusterClass"
		return reconcile.Result{}, nil
	}

	var cluster *clusterv1.Cluster
	cluster, err = util.GetOwnerCluster(ctx, r.Client, machineTemplate.ObjectMeta)
	if err != nil || cluster == nil {
		log.Info(fmt.Sprintf("%s is missing ownerRef to cluster or cluster does not exist %s/%s",
			machineTemplate.Kind, machineTemplate.Namespace, machineTemplate.Name))
		return reconcile.Result{Requeue: true}, nil
	}
	machineTemplate.Status.OwnerType = cluster.Kind

	log = log.WithValues("Cluster", klog.KObj(cluster))

	// Requeue if cluster has no infrastructure yet.
	if cluster.Spec.InfrastructureRef == nil {
		return reconcile.Result{Requeue: true}, nil
	}

	libvirtCluster := &infrav1alpha1.LibvirtCluster{}

	libvirtClusterName := client.ObjectKey{
		Namespace: machineTemplate.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, libvirtClusterName, libvirtCluster); err != nil {
		log.Info("LibvirtCluster is not available yet")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("LibvirtCluster", klog.KObj(libvirtCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Create the scope.
	secretManager := secrets.NewSecretManager(log, r.Client, r.APIReader)
	libvirtUri, _, err := getAndValidateLibvirtConnectionString(ctx, req.Namespace, libvirtCluster, secretManager)
	if err != nil {
		return libvirtUriErrorResult(ctx, err, machineTemplate, infrav1alpha1.LibvirtURIAvailableCondition, r.Client)
	}

	lc, err := r.LibvirtclientFactory.NewClient(libvirtUri)

	machineTemplateScope, err := scope.NewLibvirtMachineTemplateScope(scope.LibvirtMachineTemplateScopeParams{
		Client:                 r.Client,
		Logger:                 &log,
		LibvirtMachineTemplate: machineTemplate,
		LibvirtClient:          lc,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any LibvirtMachine changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, libvirtclient.ErrUnauthorized) {
			conditions.MarkFalse(machineTemplate, infrav1alpha1.LibvirtURIAvailableCondition, infrav1alpha1.LibvirtURIInvalidReason, clusterv1.ConditionSeverityError, "wrong Libvirt token")
		} else {
			conditions.MarkTrue(machineTemplate, infrav1alpha1.LibvirtURIAvailableCondition)
		}
	}()

	return reconcile.Result{}, r.reconcile(ctx, machineTemplateScope)
}

func (r *LibvirtMachineTemplateReconciler) reconcile(ctx context.Context, machineTemplateScope *scope.LibvirtMachineTemplateScope) error {
	LibvirtMachineTemplate := machineTemplateScope.LibvirtMachineTemplate

	// reconcile machine template
	if err := machinetemplate.NewService(machineTemplateScope).Reconcile(ctx); err != nil {
		return fmt.Errorf("failed to reconcile machine template for LibvirtMachineTemplate %s/%s: %w",
			LibvirtMachineTemplate.Namespace, LibvirtMachineTemplate.Name, err)
	}

	return nil
}

func (r *LibvirtMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1alpha1.LibvirtMachineTemplate{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// hasOwnerClusterClass returns whether the object has a ClusterClass as owner.
func hasOwnerClusterClass(obj metav1.ObjectMeta) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "ClusterClass" {
			return true
		}
	}
	return false
}
