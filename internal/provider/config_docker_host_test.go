package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestMakeContainerCreateClientWaitsForDockerHost(t *testing.T) {
	var pings atomic.Int32
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_ping") && pings.Add(1) <= 2 {
			// A host still booting
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Api-Version", "1.44")
		_, _ = w.Write([]byte(`{"ApiVersion":"1.44"}`))
	}))
	defer daemon.Close()

	config := &ProviderConfig{DefaultConfig: &Config{Host: "tcp://127.0.0.1:1"}}
	container := resourceDockerContainer().TestResourceData()
	host := "tcp://" + strings.TrimPrefix(daemon.URL, "http://")
	if err := container.Set("docker_host", host); err != nil {
		t.Fatal(err)
	}

	dockerClient, err := makeContainerCreateClient(t.Context(), container, config)
	if err != nil {
		t.Fatal(err)
	}
	if got := dockerClient.DaemonHost(); got != host {
		t.Errorf("got %q, want %q", got, host)
	}
	if got := pings.Load(); got < 3 {
		t.Errorf("pinged %d times, want a retry after the failures", got)
	}
}
