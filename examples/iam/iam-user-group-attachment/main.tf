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

variable "membership_user" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the membership user name. Used by negative tests."

  validation {
    condition = (
      var.membership_user == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.membership_user))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "membership_group" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the membership group name. Used by negative tests."

  validation {
    condition = (
      var.membership_group == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.membership_group))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "membership_ds_user_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the membership data source user_name. Used by negative tests."

  validation {
    condition = (
      var.membership_ds_user_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.membership_ds_user_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"
  user_name   = "${local.name_prefix}-user"
  group_name  = "${local.name_prefix}-group"
}

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  name = local.group_name
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = coalesce(var.membership_user, ctyun_iam_user.test.name)
  group = coalesce(var.membership_group, ctyun_iam_group.test.name)
}

data "ctyun_iam_user_group_membership" "test" {
  user_name = coalesce(var.membership_ds_user_name, ctyun_iam_user.test.name)

  depends_on = [ctyun_iam_user_group_membership.test]
}

data "ctyun_iam_group" "test" {
  group_name = ctyun_iam_group.test.name

  depends_on = [ctyun_iam_user_group_membership.test]
}

output "user_name" {
  value = ctyun_iam_user.test.name
}

output "user_arn" {
  value = ctyun_iam_user.test.arn
}

output "group_name" {
  value = ctyun_iam_group.test.name
}

output "group_arn" {
  value = ctyun_iam_group.test.arn
}

output "group_unique_id" {
  value = ctyun_iam_group.test.unique_id
}

output "membership_id" {
  value = ctyun_iam_user_group_membership.test.id
}

output "membership_user" {
  value = ctyun_iam_user_group_membership.test.user
}

output "membership_group" {
  value = ctyun_iam_user_group_membership.test.group
}

output "expected_membership_id" {
  value = "${ctyun_iam_user.test.name}/${ctyun_iam_group.test.name}"
}

output "membership_ds_id" {
  value = data.ctyun_iam_user_group_membership.test.id
}

output "membership_ds_group_count" {
  value = length(data.ctyun_iam_user_group_membership.test.groups_for_user)
}

output "membership_ds_group_names" {
  value = toset(data.ctyun_iam_user_group_membership.test.groups_for_user[*].group_name)
}

output "membership_ds_group_arns" {
  value = toset(data.ctyun_iam_user_group_membership.test.groups_for_user[*].arn)
}

output "membership_ds_group_ids" {
  value = toset(data.ctyun_iam_user_group_membership.test.groups_for_user[*].group_id)
}

output "group_ds_user_count" {
  value = length(data.ctyun_iam_group.test.users)
}

output "group_ds_user_names" {
  value = toset(data.ctyun_iam_group.test.users[*].user_name)
}
