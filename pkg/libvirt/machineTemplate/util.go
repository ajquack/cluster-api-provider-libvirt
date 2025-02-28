package machinetemplate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// GetCPUQuantityFromInt returns a resource quantity for CPU cores from an integer.
func GetCPUQuantityFromInt(cores int) (resource.Quantity, error) {
	return resource.ParseQuantity(fmt.Sprintf("%v", cores))
}

// GetMemoryQuantityFromInt returns a resource quantity for memory in GB from a int.
func GetMemoryQuantityFromInt(memory int) (resource.Quantity, error) {
	return resource.ParseQuantity(fmt.Sprintf("%vG", memory))
}
