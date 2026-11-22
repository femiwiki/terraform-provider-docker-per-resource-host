# terraform-provider-docker-per-resource-host

A fork of [kreuzwerker/terraform-provider-docker] that adds `docker_host` to
`docker_container`, so one provider can run containers on several hosts,
including hosts created in the same apply. Femiwiki uses it to move its app
containers between hosts ([femiwiki/femiwiki#577]).

## Branches

- `main` holds only what keeps the fork up to date: this README, the
  [Maintain] workflow and the GoReleaser config.
- `patches` is the latest upstream release plus the fork's commits, kept as
  few and as small as possible:
  - `docker_host` on `docker_container`, which replaces the provider's `host`
    for that container
  - on create, a retry until that host's Docker daemon answers
- The other branches come from upstream and are not used.

A change to the provider is a pull request to `patches`. It is not merged
upstream: [kreuzwerker/terraform-provider-docker#756] proposed the same and
was closed.

## Releases

[Maintain] runs daily and on demand. It rebases `patches` onto the latest
upstream release, runs the fork's tests, and when `patches` has changed since
the last release, tags `<upstream version>-fw.<n>` and publishes linux
`amd64` and `arm64` builds. If the commits do not apply onto a new upstream
release, the run fails and `patches` is left as it was.

Releases are not signed and not on the Terraform or OpenTofu registry.
OpenTofu installs them from a [filesystem mirror]:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/path/to/mirror"
    include = ["femiwiki.invalid/femiwiki/docker"]
  }
  direct {
    exclude = ["femiwiki.invalid/femiwiki/docker"]
  }
}
```

with each release's zip files in
`/path/to/mirror/femiwiki.invalid/femiwiki/docker/`, and

```hcl
terraform {
  required_providers {
    docker = {
      source  = "femiwiki.invalid/femiwiki/docker"
      version = "4.7.0-fw.1"
    }
  }
}
```

[kreuzwerker/terraform-provider-docker]: https://github.com/kreuzwerker/terraform-provider-docker
[femiwiki/femiwiki#577]: https://github.com/femiwiki/femiwiki/issues/577
[Maintain]: .github/workflows/maintain.yml
[kreuzwerker/terraform-provider-docker#756]: https://github.com/kreuzwerker/terraform-provider-docker/pull/756
[filesystem mirror]: https://opentofu.org/docs/cli/config/config-file/#filesystem_mirror
