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

variable "role_arn" {
  type        = string
  description = "IAM role ARN to AssumeRole into before managing resources."

  validation {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/.+", var.role_arn))
    error_message = "role_arn must be a CTYUN IAM role ARN (arn:ctyun:iam::<account>:role/<name>)."
  }
}

provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key

  assume_role {
    role_arn        = var.role_arn
    session_name    = "tf-cli-assume-role-full"
    duration        = "1h"
    external_id     = "terraform-session-123450987"
    source_identity = "tf-cli-source"
    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Sid      = "AllowAccountSummaryOnly"
        Effect   = "Allow"
        Action   = ["iam:GetAccountSummary"]
        Resource = "*"
      }]
    })
  }
}

data "ctyun_iam_account_summary" "test" {}

output "summary_id" {
  value = data.ctyun_iam_account_summary.test.id
}

output "users" {
  value = data.ctyun_iam_account_summary.test.users
}
