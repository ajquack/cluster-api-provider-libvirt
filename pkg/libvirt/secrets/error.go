package secrets

import (
	"fmt"
)

// ResolveSecretRefError is returned when the  secret
// for a host is defined but cannot be found.
type ResolveSecretRefError struct {
	Message string
}

func (e ResolveSecretRefError) Error() string {
	return fmt.Sprintf("Secret doesn't exist %s",
		e.Message)
}

// HCloudTokenValidationError is returned when the HCloud token in Hetzner secret is invalid.
type LibvirtConnectionStringError struct{}

func (e LibvirtConnectionStringError) Error() string {
	return "libvirt connection string cannot be an empty string"
}
