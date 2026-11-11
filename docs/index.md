---
subcategory: ""
page_title: "Provider: CTYUN OOS"
description: |-
  The CTYUN OOS Provider is used to manage OOS S3, IAM, and CloudTrail resources.
---

# CTYUN OOS Provider

The CTYUN OOS Provider is used to manage OOS S3, IAM, and CloudTrail resources.

## Example Usage

```terraform
terraform {
  required_providers {
    ctyun = {
      source = "ctyun-oos-lab/ctyunoos"
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

For registry and local filesystem mirror installation, see the [installation and deployment guide](guides/installing.md).

## Authentication and Provider Configuration

This section describes how to configure access credentials, service endpoints, and AssumeRole for the CTYUN OOS Terraform Provider.

### Authentication Method Priority

When configuring the OOS Provider, you typically only need to choose one authentication method. The Provider resolves credentials and service endpoints according to the following rules:

1. **Static Configuration**: Parameters explicitly set in the provider block take the highest priority, including directly entered values and values referenced through Terraform variables.
2. **Environment Variables**: The Provider only reads from `CTYUNOOS_ACCESS_KEY` / `CTYUNOOS_SECRET_KEY` / `CTYUNOOS_SESSION_TOKEN` when credentials are not set in the provider block. For services not specified in the `endpoints` block, it then reads from the corresponding `CTYUNOOS_ENDPOINT`.
3. **AssumeRole (`assume_role`) with Existing Base Credentials**: If a valid `assume_role.role_arn` is configured, the Provider uses the base credentials obtained through the methods described above to call IAM `AssumeRole`, and then uses the returned temporary AK/SK/Token to access OOS resources. `assume_role` is not an independent source of base credentials; you must still provide access keys capable of assuming the role through static configuration or environment variables first.

### Security Recommendations

To improve security in production environments, it is recommended to:

- Use environment variables, Terraform variable files, or a secrets management system to inject AK/SK, avoiding hardcoding plaintext in `.tf` files.
- When cross-account or fine-grained permissions are required, use IAM `AssumeRole` (`assume_role`) to exchange sub-user long-term keys for time-limited temporary credentials.

### Supported Authentication and Configuration Methods

| Method | Description | Use Case |
| --- | --- | --- |
| Static configuration (`access_key`) | Explicitly specify AK/SK in the provider block; parameter values can be entered directly or referenced through Terraform variables; if using an existing STS identity, also configure the optional `security_token`; service addresses are configured through the optional `endpoints` block. | Local debugging, quick validation. |
| Environment variables | Inject AK/SK (and optional `security_token`) and service endpoints via `CTYUNOOS_*`; takes effect when the provider block is not written. | CI/CD, local shell, container runtime. |
| AssumeRole (`assume_role`) | Use base credentials to call IAM `AssumeRole`, then use the role's temporary credentials to access resources. | Multi-account, least privilege, CI/CD. |

### Static Configuration

Static configuration refers to directly defining access credentials in the provider block, along with optional service endpoints.

#### Parameters

| Name | Description |
| --- | --- |
| `access_key` | Access Key ID. |
| `secret_key` | Access Key Secret. |
| `security_token` (Optional) | When using an existing STS temporary identity, the Security Token configured alongside the temporary AK/SK; not required for long-term keys. |
| `endpoints` | API addresses for each service; configure this block at most once. See the `endpoints` parameter description below for details. |

**Note:** `access_key` and `secret_key` must be provided as a pair; you cannot configure only one of them. If using an STS temporary identity, provide the temporary AK, temporary SK, and `security_token` simultaneously.

#### `endpoints`

| Service | Configuration Item | Example Format |
| --- | --- | --- |
| S3 | `endpoints.s3` | `https://<s3-host>:<port>` |
| IAM | `endpoints.iam` | `https://<iam-host>:<port>` |
| CloudTrail | `endpoints.cloudtrail` | `https://<trail-host>:<port>` |

Example:

```terraform
provider "ctyun" {
  access_key = "<Access Key ID>"
  secret_key = "<Access Key Secret>"

  # If using an STS temporary identity, security_token must be configured.
  security_token = "<STS Token>"

  endpoints {
    s3         = "https://<s3-host>:<port>"
    iam        = "https://<iam-host>:<port>"
    cloudtrail = "https://<trail-host>:<port>"
  }
}
```

### Environment Variables

When corresponding fields are not explicitly configured in the provider block, the Provider reads access credentials and service endpoints from the following environment variables. This approach is ideal for injecting secrets in CI/CD pipelines, local shells, or containers, while keeping the configuration file as `provider "ctyun" {}`.

#### Credentials

| Environment Variable | Provider Parameter | Description |
| --- | --- | --- |
| `CTYUNOOS_ACCESS_KEY` | `access_key` | Access Key ID. |
| `CTYUNOOS_SECRET_KEY` | `secret_key` | Access Key Secret. |
| `CTYUNOOS_SESSION_TOKEN` (Optional) | `security_token` | Security Token when using an existing STS temporary identity; not required for long-term keys. |

**Note:** `CTYUNOOS_ACCESS_KEY` and `CTYUNOOS_SECRET_KEY` must be provided as a pair. If using an STS temporary identity, set the temporary AK, temporary SK, and `security_token` simultaneously. AK, SK and Token are treated as a single credential set: if any one of them is already configured in the provider block, the entire set will not be backfilled from environment variables.

#### Endpoints

| Service | Environment Variable | Corresponding Provider Configuration |
| --- | --- | --- |
| S3 | `CTYUNOOS_ENDPOINT` | `endpoints.s3` |
| IAM | `CTYUNOOS_ENDPOINT_IAM` | `endpoints.iam` |
| CloudTrail | `CTYUNOOS_ENDPOINT_TRAIL` | `endpoints.cloudtrail` |

Example 1: Linux

```console
export CTYUNOOS_ACCESS_KEY="<Access Key ID>"
export CTYUNOOS_SECRET_KEY="<Access Key Secret>"
# If using an STS temporary identity, CTYUNOOS_SESSION_TOKEN must be configured.
export CTYUNOOS_SESSION_TOKEN="<STS Token>"

export CTYUNOOS_ENDPOINT="https://<s3-host>:<port>"
export CTYUNOOS_ENDPOINT_IAM="https://<iam-host>:<port>"
export CTYUNOOS_ENDPOINT_TRAIL="https://<trail-host>:<port>"
```

Example 2: Windows (PowerShell)

```console
$env:CTYUNOOS_ACCESS_KEY = "<Access Key ID>"
$env:CTYUNOOS_SECRET_KEY = "<Access Key Secret>"
$env:CTYUNOOS_SESSION_TOKEN = "<STS Token>"

$env:CTYUNOOS_ENDPOINT = "https://<s3-host>:<port>"
$env:CTYUNOOS_ENDPOINT_IAM = "https://<iam-host>:<port>"
$env:CTYUNOOS_ENDPOINT_TRAIL = "https://<trail-host>:<port>"
```

The provider block can be left empty:

```terraform
provider "ctyun" {}
```

You can also specify only some fields in the provider block (for example, only `assume_role`), while the remaining credentials and endpoints are still provided by environment variables.

### Using Variables with Static Configuration

Terraform variable files serve as a supplementary approach to static Provider configuration, providing values for parameters in the provider block. This is typically split into `variables.tf`, `provider.tf`, and `terraform.tfvars` (or a custom `-var-file`). Values defined in variable files ultimately still flow into the provider block (for example, `access_key = var.oos_access_key`), so they share the same priority as static values written directly into the Provider block.

#### Define Variables (`variables.tf`)

```terraform
variable "oos_access_key" {
  description = "OOS Access Key"
  type        = string
  sensitive   = true
}

variable "oos_secret_key" {
  description = "OOS Secret Key"
  type        = string
  sensitive   = true
}

variable "oos_security_token" {
  description = "OOS STS Security Token (optional)"
  type        = string
  sensitive   = true
  default     = null
}

variable "oos_endpoints" {
  description = "OOS service endpoints (Optional)"
  type = object({
    s3         = optional(string) # OOS API
    iam        = optional(string) # IAM API
    cloudtrail = optional(string) # CloudTrail
  })
  default = {}
}
```

#### Configure Provider (`provider.tf`)

```terraform
provider "ctyun" {
  access_key = var.oos_access_key
  secret_key = var.oos_secret_key

  # If using an STS temporary identity, security_token must be configured.
  security_token = var.oos_security_token

  endpoints {
    s3         = var.oos_endpoints.s3
    iam        = var.oos_endpoints.iam
    cloudtrail = var.oos_endpoints.cloudtrail
  }
}
```

#### Using Variable Files (`terraform.tfvars`)

```hcl
# terraform.tfvars
oos_access_key = "AKXXXXXXXXXXXXXXXXXX"
oos_secret_key = "your_super_secret_key_here"
# If using an STS temporary identity, oos_security_token must be configured.
oos_security_token = "<STS Token>"

oos_endpoints = {
  s3         = "https://<s3-host>:<port>"
  iam        = "https://<iam-host>:<port>"
  cloudtrail = "https://<trail-host>:<port>"
}
```

You can also use a custom file or pass parameters via the command line.

Custom file:

```console
terraform plan -var-file="prod.tfvars"
```

Pass parameters via the command line:

```console
terraform plan \
  -var='oos_access_key=<Access Key ID>' \
  -var='oos_secret_key=<Access Key Secret>' \
  -var='oos_endpoints={s3="https://<s3-host>:<port>",iam="https://<iam-host>:<port>",cloudtrail="https://<trail-host>:<port>"}'
```

### AssumeRole

AssumeRole is suitable for scenarios where you need to separate permissions from the calling identity, perform cross-account access, or use restricted temporary sessions in CI/CD. When role information is configured, the Provider first uses the permanent (or current) AK/SK to call IAM `AssumeRole` to obtain temporary AK/SK/Token, and then uses these temporary credentials to manage resources. If `AssumeRole` fails, the process aborts and reports an error.

Before enabling `AssumeRole`, you must configure the IAM service endpoint, either by specifying it in `endpoints.iam` or by setting the `CTYUNOOS_ENDPOINT_IAM` environment variable; each hop in the role chain calls IAM through this endpoint.

**Note:**

- `AssumeRole` is only initiated when a non-empty `role_arn` is configured; if `role_arn` is missing or an empty string, role assumption is skipped and base credentials are used directly.
- You can configure multiple `assume_role` blocks to represent a role chain: roles are assumed in sequence, and the temporary credentials from the final hop are used; if any step in the chain fails, the entire operation fails.
- When you already hold temporary credentials, it is not recommended to configure `assume_role` unless you need to continue the role chain.

#### Parameters

Base credentials must still be provided through static configuration (Terraform variable assignment may be used) or environment variables. The `assume_role` block fields are as follows:

| Name | Type | Description | Required |
| --- | --- | --- | --- |
| `role_arn` | String | IAM role ARN to assume, in a format similar to `arn:ctyun:iam::<account-id>:role/<role-name>`. If not set or empty, `AssumeRole` is not called. | No |
| `session_name` | String | Session name. If not set, auto-generated as `terraform-oos-<UnixNano>`. | No |
| `external_id` | String | External ID. No default value. | No |
| `duration` | String | Session duration. If not configured, this parameter is not sent and the server handles it with a default of 1 hour. Valid range is 15 minutes to 12 hours (inclusive). Must include a time unit (e.g., `s`, `m`, `h`). Examples: `"3600s"`, `"1h"`. | No |
| `policy` | String (JSON) | Session policy; intersected with role permissions to further restrict temporary credential permissions. | No |
| `source_identity` | String | Source identity identifier. No default value. | No |

Example:

```terraform
provider "ctyun" {
  access_key = var.oos_access_key
  secret_key = var.oos_secret_key

  endpoints {
    iam = "https://<iam-host>:<port>"
    s3  = "https://<s3-host>:<port>"
  }

  assume_role {
    role_arn     = "arn:ctyun:iam::1234567890123:role/example-role"
    session_name = "terraform-session"
    external_id  = "terraform-session-123450987"
    duration     = "3600s"

    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [
        {
          Sid    = "AllowS3Scoped"
          Effect = "Allow"
          Action = ["oos:ListBucket", "oos:GetObject"]
          Resource = [
            "arn:ctyun:oos:::my-terraform-bucket",
            "arn:ctyun:oos:::my-terraform-bucket/*"
          ]
        }
      ]
    })
  }
}
```

It is recommended to place AK/SK in variable files, and only keep `assume_role` and `endpoints` in the configuration:

```terraform
provider "ctyun" {
  access_key = var.oos_access_key
  secret_key = var.oos_secret_key

  endpoints {
    iam = var.oos_endpoints.iam
    s3  = var.oos_endpoints.s3
  }

  assume_role {
    role_arn     = "arn:ctyun:iam::1234567890123:role/example-role"
    session_name = "terraform-session"
    duration     = "1h"
  }
}
```

#### Role Chain Example

```terraform
provider "ctyun" {
  access_key = var.oos_access_key
  secret_key = var.oos_secret_key

  assume_role {
    role_arn     = "arn:ctyun:iam::1111111111111:role/role-a"
    session_name = "terraform-session-a"
  }

  assume_role {
    role_arn     = "arn:ctyun:iam::2222222222222:role/role-b"
    session_name = "terraform-session-b"
  }
}
```

**Note:** Resource API calls use the temporary credentials from the final hop (`role-b`).

### Authentication Configuration Verification

`terraform validate` only checks configuration syntax and schema, and cannot verify whether remote credentials are valid.

Execute in the working directory that contains OOS resources or data sources:

```console
terraform plan
```

Terraform performs remote calls to resources or data sources actually involved during the planning process to verify whether the corresponding credentials and endpoints are available; if the configuration does not contain resources or data sources that require access to OOS, `terraform plan` alone cannot prove that the credentials are valid.

## Argument Reference

The following arguments are supported in the Provider block.

* `access_key` - (Optional) Access key used for OOS API requests. Set `CTYUNOOS_ACCESS_KEY` when no Provider credential arguments are configured. This argument must be configured with `secret_key`.
* `secret_key` - (Optional) Secret key used for OOS API requests. Set `CTYUNOOS_SECRET_KEY` when no Provider credential arguments are configured. This argument must be configured with `access_key`.
* `security_token` - (Optional) Session token for temporary credentials. Set `CTYUNOOS_SESSION_TOKEN` when no Provider credential arguments are configured.
* `assume_role` - (Optional) One or more blocks that define IAM roles to assume. The block is documented below.
* `endpoints` - (Optional) A block that configures OOS service endpoints. At least one endpoint must be configured through this block or the endpoint environment variables. The block is documented below.
* `max_retries` - (Optional) Maximum number of API request attempts, including the initial attempt. The default is `5`.
* `user_agent` - (Optional) Product details to use in the User-Agent string sent in all CTYUN OOS API calls. This replaces the default `terraform-provider-ctyunoos/<version>` identifier.

### `assume_role` Configuration Block

The following arguments are supported in each `assume_role` block.

* `role_arn` - (Required) ARN of the CTYUN IAM role to assume.
* `duration` - (Optional) Duration of the role session in Go duration format. Valid values are from `15m` through `12h` inclusive.
* `external_id` - (Optional) External identifier passed to the IAM `AssumeRole` request.
* `policy` - (Optional) JSON policy that further restricts the permissions of the assumed role session.
* `session_name` - (Optional) Name of the role session. A session name is generated when this argument is omitted.
* `source_identity` - (Optional) Source identity passed to the IAM `AssumeRole` request.

### `endpoints` Configuration Block

The following optional arguments are supported in the `endpoints` block.

* `s3` - OOS S3 endpoint URL. Set `CTYUNOOS_ENDPOINT` when this argument is not configured.
* `iam` - OOS IAM endpoint URL. Set `CTYUNOOS_ENDPOINT_IAM` when this argument is not configured.
* `cloudtrail` - OOS CloudTrail endpoint URL. Set `CTYUNOOS_ENDPOINT_TRAIL` when this argument is not configured.
