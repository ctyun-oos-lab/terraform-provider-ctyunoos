# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}

variable "access_key" {
  type        = string
  description = "Source IAM user access key ID (non-root)."
}

variable "secret_key" {
  type        = string
  sensitive   = true
  description = "Source IAM user secret access key."
}

variable "role_a_arn" {
  type        = string
  description = "First hop IAM role ARN in the AssumeRole chain."

  validation {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/.+", var.role_a_arn))
    error_message = "role_a_arn must be a CTYUN IAM role ARN."
  }
}

variable "role_b_arn" {
  type        = string
  description = "Second hop IAM role ARN in the AssumeRole chain."

  validation {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/.+", var.role_b_arn))
    error_message = "role_b_arn must be a CTYUN IAM role ARN."
  }
}

# Chain: source user -> role A -> role B. Resource calls use B's temporary credentials.
provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key

  assume_role {
    role_arn     = var.role_a_arn
    session_name = "tf-cli-assume-role-a"
    duration     = "3600s"
  }

  assume_role {
    role_arn     = var.role_b_arn
    session_name = "tf-cli-assume-role-b"
    duration     = "3600s"
  }
}

data "ctyun_iam_account_summary" "test" {}

output "summary_id" {
  value = data.ctyun_iam_account_summary.test.id
}

output "users" {
  value = data.ctyun_iam_account_summary.test.users
}

output "role_a_arn" {
  value = var.role_a_arn
}

output "role_b_arn" {
  value = var.role_b_arn
}
