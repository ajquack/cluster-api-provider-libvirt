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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/secrets"
)

const (
	secretErrorRetryDelay = time.Second * 10
)

// LibvirtClusterReconciler reconciles a LibvirtCluster object
type LibvirtClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LibvirtCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *LibvirtClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha1.LibvirtCluster{}).
		Named("libvirtcluster").
		Complete(r)
}

func getAndValidateLibvirtConnectionString(ctx context.Context, namespace string, LibvirtCluster *infrav1alpha1.LibvirtCluster, secretManager *secrets.SecretManager) (string, *corev1.Secret, error) {
	// retrieve libvirt secret
	secretNamspacedName := types.NamespacedName{Namespace: namespace, Name: LibvirtCluster.Spec.LibvirtSecret.Name}

	libvirtSecret, err := secretManager.AcquireSecret(
		ctx,
		secretNamspacedName,
		LibvirtCluster,
		false,
		LibvirtCluster.DeletionTimestamp.IsZero(),
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil, &secrets.ResolveSecretRefError{Message: fmt.Sprintf("The libvirt secret %s does not exist", secretNamspacedName)}
		}
		return "", nil, err
	}

	libvirtUri := string(libvirtSecret.Data[LibvirtCluster.Spec.LibvirtSecret.Key.LibvirtURI])

	// Validate uri
	if libvirtUri == "" {
		return "", nil, &secrets.LibvirtConnectionStringError{}
	}

	return libvirtUri, libvirtSecret, nil
}

func reconcileRateLimit(setter conditions.Setter, rateLimitWaitTime time.Duration) bool {
	condition := conditions.Get(setter, infrav1alpha1.LibvirtAPIReachableCondition)
	if condition != nil && condition.Status == corev1.ConditionFalse {
		if time.Now().Before(condition.LastTransitionTime.Time.Add(rateLimitWaitTime)) {
			// Not yet timed out, reconcile again after timeout
			// Don't give a more precise requeueAfter value to not reconcile too many
			// objects at the same time
			return true
		}
		// Wait time is over, we continue
		conditions.MarkTrue(setter, infrav1alpha1.LibvirtAPIReachableCondition)
	}
	return false
}

func libvirtUriErrorResult(
	ctx context.Context,
	err error,
	setter conditions.Setter,
	conditionType clusterv1.ConditionType,
	client client.Client,
) (res ctrl.Result, reterr error) {
	switch err.(type) {
	// In the event that the reference to the secret is defined, but we cannot find it
	// we requeue the host as we will not know if they create the secret
	// at some point in the future.
	case *secrets.ResolveSecretRefError:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1alpha1.LibvirtSecretUnreachableReason,
			clusterv1.ConditionSeverityError,
			"could not find HetznerSecret",
		)
		res = ctrl.Result{RequeueAfter: secretErrorRetryDelay}

	// No need to reconcile again, as it will be triggered as soon as the secret is updated.
	case *secrets.LibvirtConnectionStringError:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1alpha1.LibvirtURIInvalidReason,
			clusterv1.ConditionSeverityError,
			"invalid or not specified hcloud token in Hetzner secret",
		)

	default:
		conditions.MarkFalse(setter,
			conditionType,
			infrav1alpha1.LibvirtURIInvalidReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return reconcile.Result{}, fmt.Errorf("an unhandled failure occurred with the Hetzner secret: %w", err)
	}
	conditions.SetSummary(setter)
	if err := client.Status().Update(ctx, setter); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update: %w", err)
	}

	return res, err
}
