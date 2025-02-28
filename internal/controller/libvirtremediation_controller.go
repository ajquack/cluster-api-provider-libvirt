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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
	libvirtremediation "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/remediation"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
)

// LibvirtRemediationReconciler reconciles a LibvirtRemediation object.
type LibvirtRemediationReconciler struct {
	client.Client
	RateLimitWaitTime    time.Duration
	APIReader            client.Reader
	LibvirtClientFactory libvirtclient.Factory
	WatchFilterValue     string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtremediations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtremediations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtremediations/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;update;patch

// Reconcile reconciles the LibvirtRemediation object.
func (r *LibvirtRemediationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	LibvirtRemediation := &infrav1alpha1.LibvirtRemediation{}
	err := r.Get(ctx, req.NamespacedName, LibvirtRemediation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("LibvirtRemediation", klog.KObj(LibvirtRemediation))

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, LibvirtRemediation.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the LibvirtMachine instance.
	LibvirtMachine := &infrav1alpha1.LibvirtMachine{}

	key := client.ObjectKey{
		Name:      machine.Spec.InfrastructureRef.Name,
		Namespace: machine.Spec.InfrastructureRef.Namespace,
	}

	if err := r.Get(ctx, key, LibvirtMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("LibvirtMachine", klog.KObj(LibvirtMachine))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, LibvirtMachine) {
		log.Info("LibvirtMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	libvirtCluster := &infrav1alpha1.LibvirtCluster{}

	libvirtClusterName := client.ObjectKey{
		Namespace: LibvirtMachine.Namespace,
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
		return libvirtUriErrorResult(ctx, err, LibvirtRemediation, infrav1alpha1.LibvirtURIAvailableCondition, r.Client)
	}

	lc, err := r.LibvirtClientFactory.NewClient(libvirtUri)

	remediationScope, err := scope.NewLibvirtRemediationScope(scope.LibvirtRemediationScopeParams{
		Client:             r.Client,
		Logger:             log,
		Machine:            machine,
		LibvirtMachine:     LibvirtMachine,
		LibvirtCluster:     libvirtCluster,
		LibvirtRemediation: LibvirtRemediation,
		LibvirtClient:      lc,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	conditions.MarkTrue(LibvirtRemediation, infrav1alpha1.LibvirtURIAvailableCondition)

	// Always close the scope when exiting this function so we can persist any LibvirtRemediation changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, libvirtclient.ErrUnauthorized) {
			conditions.MarkFalse(LibvirtRemediation, infrav1alpha1.LibvirtURIAvailableCondition, infrav1alpha1.LibvirtURIInvalidReason, clusterv1.ConditionSeverityError, "wrong Libvirt token")
		} else {
			conditions.MarkTrue(LibvirtRemediation, infrav1alpha1.LibvirtURIAvailableCondition)
		}

		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

		if err := remediationScope.Close(ctx, patchOpts...); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Check whether rate limit has been reached and if so, then wait.
	if wait := reconcileRateLimit(LibvirtRemediation, r.RateLimitWaitTime); wait {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if !LibvirtRemediation.ObjectMeta.DeletionTimestamp.IsZero() {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	return r.reconcileNormal(ctx, remediationScope)
}

func (r *LibvirtRemediationReconciler) reconcileNormal(ctx context.Context, remediationScope *scope.LibvirtRemediationScope) (reconcile.Result, error) {
	LibvirtRemediation := remediationScope.LibvirtRemediation

	// reconcile Libvirt remediation
	result, err := libvirtremediation.NewService(remediationScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile server for LibvirtRemediation %s/%s: %w",
			LibvirtRemediation.Namespace, LibvirtRemediation.Name, err)
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtRemediationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LibvirtRemediation{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}
