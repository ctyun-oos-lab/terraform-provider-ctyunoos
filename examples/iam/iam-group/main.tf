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

variable "group_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated IAM group name. Used by negative tests."

  validation {
    condition = (
      var.group_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.group_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  group_name = coalesce(var.group_name, "tf-acc-test-${random_id.suffix.hex}-group")
}

resource "ctyun_iam_group" "test" {
  name = local.group_name
}

data "ctyun_iam_group" "test" {
  group_name = ctyun_iam_group.test.name
}

data "ctyun_iam_groups" "test" {
  name_regex = "^${local.group_name}$"

  depends_on = [ctyun_iam_group.test]
}

output "group_arn" {
  value = ctyun_iam_group.test.arn
}

output "group_id" {
  value = ctyun_iam_group.test.id
}

output "group_name" {
  value = ctyun_iam_group.test.name
}

output "group_unique_id" {
  value = ctyun_iam_group.test.unique_id
}

output "data_group_arn" {
  value = data.ctyun_iam_group.test.arn
}

output "data_group_id" {
  value = data.ctyun_iam_group.test.group_id
}

output "data_group_name" {
  value = data.ctyun_iam_group.test.group_name
}

output "data_group_user_count" {
  value = length(data.ctyun_iam_group.test.users)
}

output "expected_group_arns" {
  value = toset([ctyun_iam_group.test.arn])
}

output "expected_group_names" {
  value = toset([local.group_name])
}

output "groups_ds_arns" {
  value = data.ctyun_iam_groups.test.arns
}

output "groups_ds_names" {
  value = data.ctyun_iam_groups.test.names
}

output "groups_ds_group_ids" {
  value = toset(data.ctyun_iam_groups.test.groups[*].group_id)
}
