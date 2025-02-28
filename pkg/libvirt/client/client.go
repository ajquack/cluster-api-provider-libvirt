package libvirtclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	utils "github.com/ajquack/cluster-api-provider-libvirt/pkg/libvirt/utils"
	libvirt "github.com/libvirt/libvirt-go"
)

const errStringUnauthorized = "(unauthorized)"

// ErrUnauthorized means that the API call is unauthorized.
var ErrUnauthorized = fmt.Errorf("unauthorized")

// Client collects all methods used by the controller in the libvirt API.
type Client interface {
	Close(ctx context.Context) error
	GetDomain(ctx context.Context, uuid string) (*libvirt.Domain, error)
	StartDomain(ctx context.Context, domain *libvirt.Domain) error
	ShutdownDomain(ctx context.Context, domain *libvirt.Domain) error
	RebootDomain(ctx context.Context, domain *libvirt.Domain) error
	DestroyDomain(ctx context.Context, domain *libvirt.Domain) error
	CreateDomain(ctx context.Context, xmlConfig string) (*libvirt.Domain, error)
	CreateCloudInitDisk(ctx context.Context, userData []byte, isoPath string) error
}

// Factory is the interface for creating new Client objects.
type Factory interface {
	NewClient(uri string) (Client, error)
}

type factory struct{}

var _ = Factory(&factory{})

// NewFactory creates a new factory for libvirt clients.
func NewFactory() Factory {
	return &factory{}
}

type realClient struct {
	conn *libvirt.Connect
	//dom  *libvirt.Domain
}

var _ Client = &realClient{}

// NewClient creates a new libvirt client.
func (f *factory) NewClient(uri string) (Client, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, err
	}
	return &realClient{conn: conn}, nil
}

func (c *realClient) GetDomain(ctx context.Context, uuid string) (*libvirt.Domain, error) {
	uuidByte, err := utils.ConvertDomainIDToUUIDByte(uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domainID to UUID: %w", err)
	}
	domain, err := c.conn.LookupDomainByUUID(uuidByte)
	if err != nil {
		if strings.Contains(err.Error(), "domain not found or unexpectedly disappeared") {
			return nil, fmt.Errorf("domain not found: %w", err)
		}
		return nil, err
	}
	return domain, nil
}

func (c *realClient) StartDomain(ctx context.Context, domain *libvirt.Domain) error {
	err := domain.Create()
	if err != nil {
		return err
	}
	return nil
}

func (c *realClient) ShutdownDomain(ctx context.Context, domain *libvirt.Domain) error {
	err := domain.Shutdown()
	if err != nil {
		return err
	}
	return nil
}

func (c *realClient) RebootDomain(ctx context.Context, domain *libvirt.Domain) error {
	err := domain.Reboot(libvirt.DOMAIN_REBOOT_DEFAULT)
	if err != nil {
		return err
	}
	return nil
}

func (c *realClient) DestroyDomain(ctx context.Context, domain *libvirt.Domain) error {
	err := domain.Destroy()
	if err != nil {
		return err
	}
	return nil
}

func (c *realClient) CreateDomain(ctx context.Context, xmlConfig string) (*libvirt.Domain, error) {
	domain, err := c.conn.DomainDefineXML(xmlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to define domain from XML: %w", err)
	}

	// Start the domain
	if err := domain.Create(); err != nil {
		// Clean up the domain if it fails to start
		domain.Undefine()
		domain.Free()
		return nil, fmt.Errorf("failed to start domain: %w", err)
	}

	return domain, nil
}

func (c *realClient) Close(ctx context.Context) error {
	_, err := c.conn.Close()
	return err
}

// CreateCloudInitDisk creates a cloud-init disk for a VM
func (c *realClient) CreateCloudInitDisk(ctx context.Context, userData []byte, isoPath string) error {
	// Create temporary files for user-data and meta-data
	userDataFile, err := os.CreateTemp("", "user-data")
	if err != nil {
		return fmt.Errorf("failed to create user-data temp file: %w", err)
	}
	defer os.Remove(userDataFile.Name())

	metaDataFile, err := os.CreateTemp("", "meta-data")
	if err != nil {
		return fmt.Errorf("failed to create meta-data temp file: %w", err)
	}
	defer os.Remove(metaDataFile.Name())

	// Write the data to the files
	if _, err := userDataFile.WriteString(userData); err != nil {
		return fmt.Errorf("failed to write user-data: %w", err)
	}
	if _, err := metaDataFile.WriteString(metaData); err != nil {
		return fmt.Errorf("failed to write meta-data: %w", err)
	}

	// Close the files to ensure data is flushed
	userDataFile.Close()
	metaDataFile.Close()

	// Generate the ISO using genisoimage
	cmd := exec.CommandContext(ctx, "genisoimage", "-output", isoPath, "-volid", "cidata",
		"-joliet", "-rock", userDataFile.Name(), metaDataFile.Name())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create cloud-init ISO: %s: %w", output, err)
	}

	return nil
}
