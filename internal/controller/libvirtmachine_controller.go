/*
Copyright 2025.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirtclient "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/client"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/scope"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/virtualMachine"
)

// LibvirtMachineReconciler reconciles a LibvirtMachine object
type LibvirtMachineReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	LibvirtClientFactory libvirtclient.Factory
	APIReader            client.Reader
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LibvirtMachine{}).
		// Named("libvirtmachine").
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1alpha1.GroupVersion.WithKind("LibvirtMachine"))),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *LibvirtMachineReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the LibvirtMachine instance
	libvirtMachine := &infrav1alpha1.LibvirtMachine{}
	err := r.Get(ctx, req.NamespacedName, libvirtMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("LibvirtMachine", klog.KObj(libvirtMachine))

	// Fetch the Machine
	machine, err := util.GetOwnerMachine(ctx, r.Client, libvirtMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, libvirtMachine) {
		log.Info("LibvirtMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	libvirtCluster := &infrav1alpha1.LibvirtCluster{}

	libvirtClusterName := client.ObjectKey{
		Namespace: libvirtMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, libvirtClusterName, libvirtCluster); err != nil {
		log.Info("LibvirtCluster is not available yet")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("LibvirtCluster", klog.KObj(libvirtCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	secretManager := secrets.NewSecretManager(log, r.Client, r.APIReader)
	libvirtUri, libvirtSecret, err := getAndValidateLibvirtConnectionString(ctx, req.Namespace, libvirtCluster, secretManager)
	if err != nil {
		return libvirtUriErrorResult(ctx, err, libvirtMachine, infrav1alpha1.LibvirtURIAvailableCondition, r.Client)
	}

	lc, err := r.LibvirtClientFactory.NewClient(libvirtUri)

	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		ClusterScopeParams: scope.ClusterScopeParams{
			Client:         r.Client,
			Logger:         log,
			Cluster:        cluster,
			LibvirtSecret:  libvirtSecret,
			LibvirtClient:  lc,
			LibvirtCluster: libvirtCluster,
			APIReader:      r.APIReader,
		},
		Machine:        machine,
		LibvirtMachine: libvirtMachine,
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any HCloudMachine changes.
	defer func() {
		if reterr != nil && errors.Is(reterr, libvirtclient.ErrUnauthorized) {
			conditions.MarkFalse(libvirtMachine, infrav1alpha1.LibvirtURIAvailableCondition, infrav1alpha1.LibvirtURIInvalidReason, clusterv1.ConditionSeverityError, "wrong libvirt uri")
		} else {
			conditions.MarkTrue(libvirtMachine, infrav1alpha1.LibvirtURIAvailableCondition)
		}

		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Delete the LibvirtMachine if the deletion timestamp is not zero
	if !libvirtMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	return r.reconcileNormal(ctx, machineScope)
}

func (r *LibvirtMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	libvirtMachine := machineScope.LibvirtMachine

	// Delete the instance
	result, err := virtualMachine.NewService(machineScope).DeleteDomain(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to delete LibvirtMachine %s/%s: %v", libvirtMachine.Namespace, libvirtMachine.Name, err)
	}
	emptyResult := reconcile.Result{}
	if result != emptyResult {
		return result, nil
	}

	// Remove the finalizers
	controllerutil.RemoveFinalizer(libvirtMachine, infrav1alpha1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *LibvirtMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope) (reconcile.Result, error) {
	libvirtMachine := machineScope.LibvirtMachine

	// If the LibvirtMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(libvirtMachine, infrav1alpha1.MachineFinalizer)

	// Create the instance
	result, err := virtualMachine.NewService(machineScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile LibvirtMachine %s/%s: %v", libvirtMachine.Namespace, libvirtMachine.Name, err)
	}
	emptyResult := reconcile.Result{}
	if result != emptyResult {
		return result, nil
	}

	return reconcile.Result{}, nil
}
