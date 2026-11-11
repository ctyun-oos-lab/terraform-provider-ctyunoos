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

provider "ctyun" {}

variable "role_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated IAM role name. Used by negative tests."

  validation {
    condition = (
      var.role_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.role_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "description" {
  type        = string
  default     = "Role managed by Terraform"
  description = "IAM role description."
}

variable "max_session_duration" {
  type        = number
  default     = 3600
  description = "Maximum session duration in seconds."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  role_name = coalesce(var.role_name, "tf-acc-test-${random_id.suffix.hex}-role")
  assume_role_policy = {
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = "*"
    }]
  }
}

resource "ctyun_iam_role" "test" {
  name                 = local.role_name
  description          = var.description
  max_session_duration = var.max_session_duration
  assume_role_policy   = jsonencode(local.assume_role_policy)
}

data "ctyun_iam_role" "test" {
  name = ctyun_iam_role.test.name
}

data "ctyun_iam_roles" "test" {
  name_regex = "^${local.role_name}$"

  depends_on = [ctyun_iam_role.test]
}

output "role_arn" {
  value = ctyun_iam_role.test.arn
}

output "role_id" {
  value = ctyun_iam_role.test.id
}

output "role_name" {
  value = ctyun_iam_role.test.name
}

output "role_unique_id" {
  value = ctyun_iam_role.test.unique_id
}

output "role_description" {
  value = ctyun_iam_role.test.description
}

output "role_max_session_duration" {
  value = ctyun_iam_role.test.max_session_duration
}

output "role_assume_effect" {
  value = jsondecode(ctyun_iam_role.test.assume_role_policy).Statement[0].Effect
}

output "data_role_arn" {
  value = data.ctyun_iam_role.test.arn
}

output "data_role_name" {
  value = data.ctyun_iam_role.test.name
}

output "data_role_unique_id" {
  value = data.ctyun_iam_role.test.unique_id
}

output "data_role_description" {
  value = data.ctyun_iam_role.test.description
}

output "data_role_max_session_duration" {
  value = data.ctyun_iam_role.test.max_session_duration
}

output "data_role_assume_effect" {
  value = jsondecode(data.ctyun_iam_role.test.assume_role_policy).Statement[0].Effect
}

output "expected_role_arns" {
  value = toset([ctyun_iam_role.test.arn])
}

output "expected_role_names" {
  value = toset([local.role_name])
}

output "roles_ds_arns" {
  value = data.ctyun_iam_roles.test.arns
}

output "roles_ds_names" {
  value = data.ctyun_iam_roles.test.names
}

output "roles_ds_unique_ids" {
  value = toset(data.ctyun_iam_roles.test.roles[*].unique_id)
}
