# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }

    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

variable "access_key" {
  type        = string
  description = "Source IAM user access key ID (non-root). Root AK cannot AssumeRole."
}

variable "secret_key" {
  type        = string
  sensitive   = true
  description = "Source IAM user secret access key."
}

variable "role_arn" {
  type        = string
  description = "IAM role ARN to AssumeRole into before managing resources."

  validation {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/.+", var.role_arn))
    error_message = "role_arn must be a CTYUN IAM role ARN (arn:ctyun:iam::<account>:role/<name>)."
  }
}

variable "session_name" {
  type        = string
  default     = "tf-cli-assume-role"
  description = "AssumeRole session name."

  validation {
    condition     = can(regex("^[\\w+=,.@-]{2,64}$", var.session_name))
    error_message = "session_name must be 2-64 characters matching [\\w+=,.@-]."
  }
}

variable "duration" {
  type        = string
  default     = "3600s"
  description = "AssumeRole session duration. Must include a Go duration unit (s, m, h)."

  validation {
    condition = (
      can(regex("^[0-9]+(ns|us|µs|ms|s|m|h)$", var.duration)) &&
      (
        (can(regex("m$", var.duration)) ? tonumber(replace(var.duration, "m", "")) >= 15 : false) ||
        (can(regex("h$", var.duration)) ? (tonumber(replace(var.duration, "h", "")) >= 1 && tonumber(replace(var.duration, "h", "")) <= 12) : false) ||
        (can(regex("s$", var.duration)) ? (tonumber(replace(var.duration, "s", "")) >= 900 && tonumber(replace(var.duration, "s", "")) <= 43200) : false)
      )
    )
    error_message = "duration must include a time unit and be between 15m and 12h (for example 3600s or 1h)."
  }
}

# Explicit non-root AK/SK so env root credentials are not used for AssumeRole.
provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key

  assume_role {
    role_arn     = var.role_arn
    session_name = var.session_name
    duration     = var.duration
  }
}

# Allowed by the role policy (iam:GetAccountSummary only).
# Source user cannot do this without AssumeRole — success proves temp role creds.
data "ctyun_iam_account_summary" "test" {}

output "summary_id" {
  value = data.ctyun_iam_account_summary.test.id
}

output "users" {
  value = data.ctyun_iam_account_summary.test.users
}

output "assumed_role_arn" {
  value = var.role_arn
}
