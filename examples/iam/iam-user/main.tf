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

variable "user_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated IAM user name. Used by negative tests."

  validation {
    condition = (
      var.user_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.user_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "managed_by" {
  type        = string
  default     = "terraform"
  description = "Value of the managed_by tag."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  user_name = coalesce(var.user_name, "tf-acc-test-${random_id.suffix.hex}-user")
}

resource "ctyun_iam_user" "test" {
  name = local.user_name

  tags = {
    managed_by = var.managed_by
  }
}

data "ctyun_iam_user" "test" {
  user_name = ctyun_iam_user.test.name
}

data "ctyun_iam_users" "test" {
  name_regex = "^${local.user_name}$"

  depends_on = [ctyun_iam_user.test]
}

output "user_arn" {
  value = ctyun_iam_user.test.arn
}

output "user_id" {
  value = ctyun_iam_user.test.id
}

output "user_name" {
  value = ctyun_iam_user.test.name
}

output "user_unique_id" {
  value = ctyun_iam_user.test.unique_id
}

output "managed_by" {
  value = ctyun_iam_user.test.tags["managed_by"]
}

output "data_user_arn" {
  value = data.ctyun_iam_user.test.arn
}

output "data_user_name" {
  value = data.ctyun_iam_user.test.user_name
}

output "expected_user_arns" {
  value = toset([ctyun_iam_user.test.arn])
}

output "expected_user_names" {
  value = toset([local.user_name])
}

output "users_ds_arns" {
  value = data.ctyun_iam_users.test.arns
}

output "users_ds_names" {
  value = data.ctyun_iam_users.test.names
}

output "users_ds_user_ids" {
  value = toset(data.ctyun_iam_users.test.users[*].user_id)
}
