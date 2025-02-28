package utils

import (
	"fmt"

	infrav1alpha1 "github.com/ajquack/cluster-api-provider-libvirt/api/v1alpha1"
	libvirt "github.com/libvirt/libvirt-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
)

// FindOwnerRefFromList finds the owner ref of a Kubernetes object in a list of owner refs.
func FindOwnerRefFromList(refList []metav1.OwnerReference, name, kind, apiVersion string) (ref int, found bool) {
	bGV, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		panic("object has invalid group version")
	}

	for i, curOwnerRef := range refList {
		aGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			// ignore owner ref if it has invalid group version
			continue
		}

		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if curOwnerRef.Name == name &&
			curOwnerRef.Kind == kind &&
			aGV.Group == bGV.Group {
			return i, true
		}
	}
	return 0, false
}

// RemoveOwnerRefFromList removes the owner reference of a Kubernetes object.
func RemoveOwnerRefFromList(refList []metav1.OwnerReference, name, kind, apiVersion string) []metav1.OwnerReference {
	if len(refList) == 0 {
		return refList
	}
	index, found := FindOwnerRefFromList(refList, name, kind, apiVersion)
	// if owner ref is not found, return
	if !found {
		return refList
	}

	// if it is the only owner ref, we can return an empty slice
	if len(refList) == 1 {
		return []metav1.OwnerReference{}
	}

	// remove owner ref from slice
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList = refList[:refListLen]

	return RemoveOwnerRefFromList(refList, name, kind, apiVersion)
}

// StringInList returns a boolean indicating whether strToSearch is a
// member of the string slice passed as the first argument.
func StringInList(list []string, strToSearch string) bool {
	for _, item := range list {
		if item == strToSearch {
			return true
		}
	}
	return false
}

type runtimeObjectWithConditions interface {
	conditions.Setter
	runtime.Object
}

func HandleRateLimitExceeded(obj runtimeObjectWithConditions, err error, functionName string) bool {
	if _, ok := err.(libvirt.Error); !ok {
		msg := fmt.Sprintf("exceeded libvirt rate limit with calling function %q", functionName)
		conditions.MarkFalse(
			obj,
			infrav1alpha1.LibvirtAPIReachableCondition,
			infrav1alpha1.RateLimitExceededReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			msg,
		)
		record.Warnf(obj, "RateLimitExceeded", msg)
		return true
	}
	return false
}
