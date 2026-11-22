package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceDockerHost(t *testing.T) {
	container := resourceDockerContainer().TestResourceData()
	if got := resourceDockerHost(container); got != "" {
		t.Errorf("unset docker_host: got %q, want empty", got)
	}

	if err := container.Set("docker_host", "ssh://ec2-user@i-0123456789abcdef0"); err != nil {
		t.Fatal(err)
	}
	if got := resourceDockerHost(container); got != "ssh://ec2-user@i-0123456789abcdef0" {
		t.Errorf("set docker_host: got %q", got)
	}

	// A resource without the attribute must not panic
	network := resourceDockerNetwork().TestResourceData()
	if got := resourceDockerHost(network); got != "" {
		t.Errorf("resource without docker_host: got %q, want empty", got)
	}

	if got := resourceDockerHost((*schema.ResourceData)(nil)); got != "" {
		t.Errorf("nil resource data: got %q, want empty", got)
	}
}

func TestMakeClientUsesResourceDockerHost(t *testing.T) {
	config := &ProviderConfig{
		DefaultConfig: &Config{
			Host:                     "tcp://127.0.0.1:1",
			DisableDockerDaemonCheck: true,
		},
	}

	container := resourceDockerContainer().TestResourceData()
	if err := container.Set("docker_host", "tcp://127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}

	own, err := config.MakeClient(t.Context(), container)
	if err != nil {
		t.Fatal(err)
	}
	if got := own.DaemonHost(); got != "tcp://127.0.0.1:2" {
		t.Errorf("container with docker_host: got %q", got)
	}

	shared, err := config.MakeClient(t.Context(), resourceDockerContainer().TestResourceData())
	if err != nil {
		t.Fatal(err)
	}
	if got := shared.DaemonHost(); got != "tcp://127.0.0.1:1" {
		t.Errorf("container without docker_host: got %q", got)
	}
}
