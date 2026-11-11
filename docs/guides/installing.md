---
subcategory: ""
page_title: "Install and Deploy the CTYUN OOS Provider"
description: |-
  Install the CTYUN OOS Provider from the Terraform Registry or OpenTofu Registry, or from a local filesystem mirror for offline environments.
---

# Install and Deploy the CTYUN OOS Provider

This guide describes how to install the CTYUN OOS Provider, either from the Terraform Registry or OpenTofu Registry (online), or from a local filesystem mirror for offline or isolated environments.

## Prerequisites

The CTYUN OOS Provider requires Terraform CLI 1.5.7 or later; the latest stable Terraform 1.x release is recommended.

To install Terraform, follow the [official Terraform installation instructions](https://developer.hashicorp.com/terraform/install).

## Install from the Registry

The provider is available from the [Terraform Registry](https://registry.terraform.io/providers/ctyun-oos-lab/ctyunoos/latest) and the [OpenTofu Registry](https://registry.opentofu.org/providers/ctyun-oos-lab/ctyunoos/latest).

In the root module, declare the provider with its source address and a version constraint:

```hcl
terraform {
  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}
```

Run `terraform init` to download and verify the provider:

```console
terraform init
```

Terraform downloads the provider package, verifies its checksum, and records the checksums in `.terraform.lock.hcl`. Commit the lock file so that all users and CI/CD systems use the same verified provider binary.

The following sections describe installing from a local filesystem mirror, which is intended for offline or isolated environments.

## Installing the CTYUN OOS Provider from a Local Filesystem Mirror

For offline or isolated environments that cannot reach the public registries, obtain the installation package for the appropriate version and platform from the release channel, and install it via a local filesystem mirror.

Release packages use the following naming convention:

```text
terraform-provider-ctyunoos_<VERSION>_<TARGET>.zip
```

Use the package that matches your environment.

### Linux

The following example installs version `1.0.1` for Linux AMD64:

```console
export RELEASE_DIR=/opt/terraform
export MIRROR_DIR=/opt/terraform/providers
export PROVIDER_VERSION=1.0.1
export TARGET=linux_amd64
export PACKAGE="terraform-provider-ctyunoos_${PROVIDER_VERSION}_${TARGET}.zip"

cd "$RELEASE_DIR"
sudo install -d -m 0755 "$MIRROR_DIR/registry.terraform.io/ctyun-oos-lab/ctyunoos"
sudo install -m 0644 "$PACKAGE" \
  "$MIRROR_DIR/registry.terraform.io/ctyun-oos-lab/ctyunoos/$PACKAGE"
```

- `RELEASE_DIR`: Directory containing the provider release packages (the local path where the downloaded packages are stored).
- `MIRROR_DIR`: Root directory of the local filesystem mirror from which Terraform loads providers. It must match the `path` configured for `filesystem_mirror` in `~/.terraformrc`.

Create or modify `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/opt/terraform/providers"
    include = ["registry.terraform.io/ctyun-oos-lab/ctyunoos"]
  }

  direct {
    exclude = ["registry.terraform.io/ctyun-oos-lab/ctyunoos"]
  }
}
```

### Windows

The following example installs version `1.0.1` for Windows AMD64:

```powershell
$releaseDir  = "C:\path\to\release"
$mirrorDir   = "C:\Terraform\providers"
$version     = "1.0.1"
$target      = "windows_amd64"
$package     = "terraform-provider-ctyunoos_${version}_${target}.zip"
$packagePath = Join-Path $releaseDir $package
$destination = Join-Path $mirrorDir "registry.terraform.io\ctyun-oos-lab\ctyunoos"

New-Item -ItemType Directory -Force -Path $destination | Out-Null
Copy-Item $packagePath (Join-Path $destination $package)
```

Create or modify `%APPDATA%\terraform.rc`, where `%APPDATA%` is typically `C:\Users\<Username>\AppData\Roaming`.

The Terraform CLI configuration uses HCL, and paths must use forward slashes (`/`):

```hcl
provider_installation {
  filesystem_mirror {
    path    = "C:/Terraform/providers"
    include = ["registry.terraform.io/ctyun-oos-lab/ctyunoos"]
  }

  direct {
    exclude = ["registry.terraform.io/ctyun-oos-lab/ctyunoos"]
  }
}
```

## Initialize and Verify

Explicitly declare the Provider source and version in your Terraform configuration:

```hcl
terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "1.0.1"
    }
  }
}
```

Then, in the configuration directory, run:

```console
terraform init
terraform providers
```

Terraform loads `ctyun-oos-lab/ctyunoos` from the local mirror and does not access the Terraform Registry for this Provider.

When installing from a local filesystem mirror, `terraform init` may report `(unauthenticated)`. This indicates that Terraform has not obtained the signature verification metadata normally provided by the Registry. This is expected behavior for local filesystem mirror installations and does not indicate a failure in Provider installation or loading.

## Locking Cross-Platform Provider Versions

When using the same Terraform configuration on both Linux and Windows, the local mirror must contain the Provider ZIP packages for each target platform.

The following command only generates a lock file for the Provider versions declared in the root module's `required_providers`; it does not lock or download other release versions. Multiple `-platform` parameters are used solely to record installation information for different operating systems and CPU architectures for the same version.

In the root module directory, generate the lock file using a local mirror that contains installation packages for all target platforms:

```console
terraform providers lock \
  -fs-mirror=/opt/terraform/providers \
  -platform=linux_amd64 \
  -platform=linux_arm64 \
  -platform=windows_amd64 \
  -platform=windows_arm64 \
  registry.terraform.io/ctyun-oos-lab/ctyunoos
```

Keep only the target platform parameters that are actually provided in the mirror. On Windows, replace the `-fs-mirror` path with `C:/Terraform/providers`.

Include the generated `.terraform.lock.hcl` in version control along with your Terraform configuration to ensure the same Provider version is used across different platforms.

## Upgrade, Rollback, and Automation

This section describes the local filesystem mirror workflow. If you install from the Terraform Registry, upgrading is simpler: change the version constraint in `required_providers` and run `terraform init -upgrade`; to roll back, restore the previous version constraint and run `terraform init`.

Do not overwrite an existing version when upgrading.

Deploy the new package to the mirror, update the version constraint in `required_providers`, run `terraform init -upgrade` and `terraform plan` in a controlled environment, and review and commit the new `.terraform.lock.hcl`.

Promote the release to other hosts or the shared mirror only after verification.

To roll back, restore the version constraint and `.terraform.lock.hcl` to a verified earlier version.

Retain the earlier package in the mirror until every working directory has completed the rollback.

Do not manually edit the lock file or delete `.terraform/` in production to resolve a version mismatch.

CI/CD runners should:

- Use a read-only mirror directory, or copy a verified mirror to an isolated workspace at the start of each job.
- Inject `CTYUNOOS_*` credentials through protected environment variables or a secret-management service.
- Pin `TF_CLI_CONFIG_FILE` to a pipeline-managed configuration file and retain `.terraform.lock.hcl`.
- Run `terraform init`, `terraform validate`, and a manually or policy-reviewed `terraform plan` before `terraform apply`.

## References

- [Terraform CLI Configuration and Filesystem Mirrors](https://developer.hashicorp.com/terraform/cli/config/config-file)
- [Terraform Provider Dependency Lock File](https://developer.hashicorp.com/terraform/language/files/dependency-lock)
- [terraform providers lock Command](https://developer.hashicorp.com/terraform/cli/commands/providers/lock)
