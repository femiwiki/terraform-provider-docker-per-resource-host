package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

// Setting docker_host on a container that has none in state, or changing it,
// is an in-place update: the address may change while the host stays the same.
func TestResourceDockerHostChangeIsNotReplacement(t *testing.T) {
	// A sparse state makes other computed attributes plan as replacements too;
	// those come from a config without docker_host and must stay the same.
	state := &terraform.InstanceState{
		ID:         "abc",
		Attributes: map[string]string{"id": "abc", "name": "http", "image": "sha256:0123"},
	}
	replaced := func(host string) map[string]bool {
		raw := map[string]interface{}{"name": "http", "image": "sha256:0123"}
		if host != "" {
			raw["docker_host"] = host
		}
		diff, err := resourceDockerContainer().Diff(t.Context(), state, terraform.NewResourceConfigRaw(raw), nil)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for k, a := range diff.Attributes {
			if a.RequiresNew {
				got[k] = true
			}
		}
		if host != "" && diff.Attributes["docker_host"] == nil {
			t.Fatalf("%s: docker_host does not change", host)
		}
		return got
	}

	state.Attributes["docker_host"] = ""
	want := replaced("")
	for _, host := range []string{"tcp://172.17.0.1:2375", "tcp://172.18.0.1:2376"} {
		got := replaced(host)
		if got["docker_host"] {
			t.Errorf("%s: changing docker_host replaces the container", host)
		}
		delete(got, "docker_host")
		if len(got) != len(want) {
			t.Errorf("%s: replaced %v, want %v", host, got, want)
		}
		state.Attributes["docker_host"] = host
	}
}
