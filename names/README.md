<!-- Copyright IBM Corp. 2014, 2026 -->
<!-- SPDX-License-Identifier: MPL-2.0 -->

# names

Package `names` provides service-name information that is critical to the CTYUN OOS provider working correctly. If you are unsure about a change you are making, please do not hesitate to ask!

`NOTE:` The information in `data/names_data.hcl` affects the provider, generators, documentation, and website navigation working correctly. _Please do not make any changes until you understand the table below._

The core of the `names` package is `data/names_data.hcl`, which contains HCL data describing each retained OOS service. The file is embedded at build time (`//go:embed`, see `data/read.go`) and referenced by generators when generating code. _The information it contains must be correct._ Please double-check any changes.

This fork intentionally keeps only the services the OOS provider implements. The current service list is:

* `s3` — S3 (Simple Storage)
* `iam` — IAM (Identity & Access Management)
* `cloudtrail` — CloudTrail

Consumers of `names` include:

* Package `provider` (`internal/provider`)
* Package `conns` (`internal/conns`)
* The provider generators under `internal/generate/`

After any edits to `data/names_data.hcl`, run `make gen`. Doing so regenerates code and performs checks on `data/names_data.hcl`.

## Schema

The schema below reflects the blocks and attributes actually decoded by `data/read.go`. Blocks and attributes not listed here are not used by this fork. All blocks except `names` are optional; within a block, attributes marked optional may be omitted.

```hcl
service "" { // label is the provider package name (e.g. "s3")

  sdk {
    id            = ""  // service identifier, e.g. "S3"
    arn_namespace = ""  // ARN namespace, e.g. "s3"
    client_version = 2  // AWS SDK for Go version; defaults to 2 if omitted
  }

  names {
    provider_name_upper  = "" // [REQUIRED] correctly capitalized package name
    human_friendly       = "" // [REQUIRED] human-friendly service name
    human_friendly_short = "" // short form; falls back to human_friendly if empty
    aliases              = [""] // optional name variations
  }

  // OOS custom endpoint environment variable, replacing the AWS-style variables.
  env_var {
    custom_env_var = "" // e.g. "CTYUNOOS_ENDPOINT_IAM"
  }

  endpoint_info {
    endpoint_api_call = ""  // API call used to probe the endpoint, e.g. "ListRoles"
    endpoint_template = ""  // OOS endpoint template with {region} placeholder
  }

  resource_prefix {
    actual  = "" // regexp for anomalous resource-name prefixes; takes precedence
    correct = "" // regexp for what the resource-name prefix should be
  }

  provider_package_correct = "" // service identifier / correct package name
  doc_prefix               = [""] // documentation file prefixes
  brand                    = ""   // optional service brand; used in messages
  is_global                = bool // whether the service is region-independent
}
```

## Attribute reference

| Name | Use | Description |
| --- | --- | --- |
| `id` | Code | Service identifier (`ServiceID`); uniquely identifies the service. |
| `arn_namespace` | Code | ARN namespace for the service, used when constructing/parsing ARNs. |
| `client_version` | Code | AWS SDK for Go major version used for the service. Defaults to `2`. |
| `provider_name_upper` | Code | [REQUIRED] Correctly capitalized provider package name. |
| `human_friendly` | Code | [REQUIRED] Human-friendly name; used in website navigation and error messages. |
| `human_friendly_short` | Code | Short human-friendly name without parentheticals (_e.g._, `S3`). Falls back to `human_friendly`. |
| `aliases` | Code | List of name variations for the service. |
| `custom_env_var` | Code | OOS custom endpoint environment variable (_e.g._, `CTYUNOOS_ENDPOINT`); read by the provider as-is, no fixed naming convention. |
| `endpoint_api_call` | Code | API call used to describe/probe the service endpoint. |
| `endpoint_template` | Code | OOS endpoint template containing a `{region}` placeholder, substituted per-region (used for S3 per-resource region overrides). |
| `resource_prefix.actual` | Code | Regular expression matching anomalous resource-name prefixes; takes precedence over `correct`. |
| `resource_prefix.correct` | Code | Regular expression matching what resource-name prefixes should be. |
| `provider_package_correct` | Code | Service identifier; what the provider package name should be. |
| `doc_prefix` | Code | List of prefixes for service documentation files under `website/docs/`. |
| `brand` | Code | Optional service brand used in error messages. |
| `is_global` | Code | Whether the service is global (region-independent). |

For the authoritative set of blocks and attributes, see the struct definitions in `data/read.go`.
