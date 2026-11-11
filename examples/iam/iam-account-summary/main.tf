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

variable "create_resources" {
  type        = bool
  default     = false
  description = "Create one user, group, policy, and role so account summary counters can be compared."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"
  assume_role_policy = {
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = "*"
    }]
  }
  policy_document = {
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:ListUsers"]
      Resource = "*"
    }]
  }
}

resource "ctyun_iam_user" "test" {
  count = var.create_resources ? 1 : 0

  name          = "${local.name_prefix}-user"
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  count = var.create_resources ? 1 : 0

  name = "${local.name_prefix}-group"
}

resource "ctyun_iam_policy" "test" {
  count = var.create_resources ? 1 : 0

  name   = "${local.name_prefix}-policy"
  policy = jsonencode(local.policy_document)
}

resource "ctyun_iam_role" "test" {
  count = var.create_resources ? 1 : 0

  name               = "${local.name_prefix}-role"
  assume_role_policy = jsonencode(local.assume_role_policy)
}

# No depends_on: Terraform/OpenTofu resolves this data source before the
# resources below are created, giving a "before" baseline in the same apply.
data "ctyun_iam_account_summary" "baseline" {}

data "ctyun_iam_account_summary" "test" {
  depends_on = [
    ctyun_iam_user.test,
    ctyun_iam_group.test,
    ctyun_iam_policy.test,
    ctyun_iam_role.test,
  ]
}

output "summary_id" {
  value = data.ctyun_iam_account_summary.test.id
}

output "accesskeys_per_user_quota" {
  value = data.ctyun_iam_account_summary.test.accesskeys_per_user_quota
}

output "account_accesskeys_present" {
  value = data.ctyun_iam_account_summary.test.account_accesskeys_present
}

output "attached_policies_per_group_quota" {
  value = data.ctyun_iam_account_summary.test.attached_policies_per_group_quota
}

output "attached_policies_per_role_quota" {
  value = data.ctyun_iam_account_summary.test.attached_policies_per_role_quota
}

output "attached_policies_per_user_quota" {
  value = data.ctyun_iam_account_summary.test.attached_policies_per_user_quota
}

output "groups" {
  value = data.ctyun_iam_account_summary.test.groups
}

output "users_delta" {
  value = data.ctyun_iam_account_summary.test.users - data.ctyun_iam_account_summary.baseline.users
}

output "groups_delta" {
  value = data.ctyun_iam_account_summary.test.groups - data.ctyun_iam_account_summary.baseline.groups
}

output "policies_delta" {
  value = data.ctyun_iam_account_summary.test.policies - data.ctyun_iam_account_summary.baseline.policies
}

output "roles_delta" {
  value = data.ctyun_iam_account_summary.test.roles - data.ctyun_iam_account_summary.baseline.roles
}

output "groups_per_user_quota" {
  value = data.ctyun_iam_account_summary.test.groups_per_user_quota
}

output "groups_quota" {
  value = data.ctyun_iam_account_summary.test.groups_quota
}

output "mfa_devices" {
  value = data.ctyun_iam_account_summary.test.mfa_devices
}

output "mfa_devices_inuse" {
  value = data.ctyun_iam_account_summary.test.mfa_devices_inuse
}

output "policies" {
  value = data.ctyun_iam_account_summary.test.policies
}

output "policies_quota" {
  value = data.ctyun_iam_account_summary.test.policies_quota
}

output "providers" {
  value = data.ctyun_iam_account_summary.test.providers
}

output "providers_quota" {
  value = data.ctyun_iam_account_summary.test.providers_quota
}

output "roles" {
  value = data.ctyun_iam_account_summary.test.roles
}

output "roles_quota" {
  value = data.ctyun_iam_account_summary.test.roles_quota
}

output "users" {
  value = data.ctyun_iam_account_summary.test.users
}

output "users_quota" {
  value = data.ctyun_iam_account_summary.test.users_quota
}
