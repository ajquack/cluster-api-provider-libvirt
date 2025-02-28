package utils

import (
	"fmt"
	"time"

	libvirt "github.com/libvirt/libvirt-go"
)

// GetDomainState returns the current state of the domain
func GetDomainState(domain *libvirt.Domain) (libvirt.DomainState, error) {
	state, _, err := domain.GetState()
	if err != nil {
		return 0, err
	}
	return state, nil
}

// IsDomainRunning checks if the domain state indicates a running domain
func IsStateRunning(state libvirt.DomainState) bool {
	return state == libvirt.DOMAIN_RUNNING
}

// IsDomainShutoff checks if the domain state indicates a shut off domain
func IsStateShutoff(state libvirt.DomainState) bool {
	return state == libvirt.DOMAIN_SHUTOFF
}

// IsDomainShutdown checks if the domain state indicates a domain in the process of shutting down
func IsStateShutdown(state libvirt.DomainState) bool {
	return state == libvirt.DOMAIN_SHUTDOWN
}

// IsDomainPaused checks if the domain state indicates a paused domain
func IsStatePaused(state libvirt.DomainState) bool {
	return state == libvirt.DOMAIN_PAUSED
}

// GetDomainStateString returns a human-readable representation of domain state
func GetDomainStateString(state libvirt.DomainState) string {
	switch state {
	case libvirt.DOMAIN_NOSTATE:
		return "No State"
	case libvirt.DOMAIN_RUNNING:
		return "Running"
	case libvirt.DOMAIN_BLOCKED:
		return "Blocked"
	case libvirt.DOMAIN_PAUSED:
		return "Paused"
	case libvirt.DOMAIN_SHUTDOWN:
		return "Shutting down"
	case libvirt.DOMAIN_SHUTOFF:
		return "Shut off"
	case libvirt.DOMAIN_CRASHED:
		return "Crashed"
	case libvirt.DOMAIN_PMSUSPENDED:
		return "PM suspended"
	default:
		return "Unknown"
	}
}

// WaitForDomainState waits until the domain reaches the desired state, up to maxRetries
func WaitForDomainState(domain *libvirt.Domain, desiredState libvirt.DomainState, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		state, _, err := domain.GetState()
		if err != nil {
			return err
		}

		if state == desiredState {
			return nil
		}

		// Sleep for a short time before checking again
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("domain did not reach desired state within %d retries", maxRetries)
}
