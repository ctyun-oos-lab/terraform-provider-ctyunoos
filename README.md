# Terraform Provider for CTYUN OOS

[![License: MPL-2.0](https://img.shields.io/badge/License-MPL--2.0-blue.svg)](LICENSE)
[![Provider Checks](https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/actions/workflows/provider.yml/badge.svg)](https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/actions/workflows/provider.yml)

The CTYUN OOS Provider for [Terraform](https://developer.hashicorp.com/terraform) manages OOS S3, IAM, and CloudTrail resources.

The retained service surface is:

- **S3** — buckets, objects, and their subresources (ACL, policy, lifecycle, object lock, server-side encryption, soft delete, and more)
- **IAM** — users, groups, roles, policies, access keys, and related resources
- **CloudTrail** — trails

## Upstream

This provider is a derivative work of the [HashiCorp Terraform AWS Provider](https://github.com/hashicorp/terraform-provider-aws), forked at release **v6.39.0** and licensed under the [Mozilla Public License 2.0](LICENSE) (MPL-2.0).

The fork is adapted for China Telecom Cloud OOS (Object-Oriented Storage) and retains only the S3, IAM, and CloudTrail service surface. Unlike upstream, it requires explicit OOS service endpoints and does not expose the full AWS service surface.

Original copyright and license notices are preserved in the source files; files added or substantially modified for this project carry their own copyright headers.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.5.7 or later for full functionality.
- OOS access key and secret key. See [Authentication](docs/index.md#authentication-and-provider-configuration).

## Using the Provider

The provider source address is `ctyun-oos-lab/ctyunoos` and its local name is `ctyun`. Declare it in your Terraform configuration:

```hcl
terraform {
  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}

provider "ctyun" {
  access_key = var.ctyun_access_key
  secret_key = var.ctyun_secret_key

  endpoints {
    s3 = "https://<s3-host>:<port>"
  }
}
```

## Developing the Provider

This section is for contributors who build, test, or debug the provider from source.

### Requirements

- [Go](https://go.dev/doc/install) 1.26.6 (see [`.go-version`](.go-version)).
- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.5.7 or later.
- GNU `make`. Install the lint toolchain once with `make tools`; [`tenv`](https://github.com/tofuutils/tenv) is required only for `make testcompat`.

### Build

```console
make build
```

`make build` verifies Go formatting, then installs the provider binary via `go install`. For a development build that stays in the tree, use:

```console
make go-build
```

This writes the binary under `terraform-plugin-dir/`, laid out as an unpacked local filesystem mirror that you can point at with the `filesystem_mirror` block described in the [installation guide](docs/guides/installing.md).

To build cross-platform release packages (ZIP archives and `SHA256SUMS`):

```console
make release
```

### Code generation

```console
make gen          # run all Go generators
make gen-check    # verify `make gen` produces no uncommitted changes (CI)
```

### Format and lint

```console
make fmt          # reformat Go source
make fmt-check    # verify formatting (runs in CI)
make lint         # golangci-lint + import-lint
```

### Testing

Unit tests (no live OOS access):

```console
make test                         # entire codebase
make test PKG=s3                  # one service
make test T=TestUnitExample       # one test by name
make test TEST_OUTPUT=logs/test   # save output to a timestamped log
```

Acceptance tests run against live OOS and create real resources; set OOS credentials and endpoints first (see [Authentication](docs/index.md#authentication-and-provider-configuration)):

```console
make testacc PKG=iam TESTS=TestAccIAMRole_
```

Example-based IaC tests use Terraform's `test` framework on the `.tftest.hcl` files under `examples/`:

```console
make testiac                          # standard examples
make testiac WITH_SHARED_BUCKET=1     # plus shared-bucket manual tests
```

CLI compatibility matrix (requires `tenv`):

```console
make testcompat TENV_COMPATIBILITY_MATRIX="terraform:1.12.0 tofu:1.12.6"
```

Sweepers delete leftover test resources; use only in development accounts:

```console
make sweep
```

Run the complete CI suite locally with:

```console
make ci
```

`make help` lists every target with its description. The [`GNUmakefile`](GNUmakefile) header documents the `PKG`/`K`, `T`/`TESTS`, and `TEST_OUTPUT` variables.

## Documentation

- [Provider reference](docs/index.md) — configuration, credentials, and endpoints
- [Installation guide](docs/guides/installing.md) — install from the Registry or a local filesystem mirror
- [Common use cases](docs/guides/common-use-cases.md) — end-to-end examples combining S3, IAM, and CloudTrail
- [Examples](examples/) — runnable Terraform configurations
- [Changelog](CHANGELOG.md) — release notes
- [Security](SECURITY.md) — how to report security vulnerabilities

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).
