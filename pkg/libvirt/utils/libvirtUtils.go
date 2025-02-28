package utils

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const providerIDPrefix = "libvirt://"

var (
	// ErrInvalidProviderID indicates that the providerID is invalid.
	ErrInvalidProviderID = fmt.Errorf("invalid providerID")
	// ErrNilProviderID indicates that the providerID is nil.
	ErrNilProviderID = fmt.Errorf("nil providerID")
)

// ServerIDFromProviderID returns the serverID from a providerID.
func DomainIDFromProviderID(providerID *string) (string, error) {
	if providerID == nil {
		return "", ErrNilProviderID
	}
	stringParts := strings.Split(*providerID, "://")
	if len(stringParts) != 2 || stringParts[0] == "" || stringParts[1] == "" {
		return "", ErrInvalidProviderID
	}

	// Just return the part after the prefix
	return stringParts[1], nil
}

func ConvertDomainIDToUUIDByte(serverID string) ([]byte, error) {
	uuidStr := strings.Replace(serverID, "-", "", -1)
	uuid, err := hex.DecodeString(uuidStr)
	if err != nil {
		fmt.Printf("Failed to parse UUID: %v\n", err)
		return nil, err
	}
	return uuid, nil
}
