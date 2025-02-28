package v1alpha1

type LibvirtSecretRef struct {
	// Name defines the name of the secret.
	// +kubebuilder:default=hetzner
	Name string `json:"name"`
	// Key defines the keys that are used in the secret.
	// Need to specify either HCloudToken or both HetznerRobotUser and HetznerRobotPassword.
	Key LibvirtSecretKeyRef `json:"key"`
}

type LibvirtSecretKeyRef struct {
	// LibvirtURI is the key in the secret that contains the libvirt URI.
	LibvirtURI string `json:"libvirtURI"`
}
