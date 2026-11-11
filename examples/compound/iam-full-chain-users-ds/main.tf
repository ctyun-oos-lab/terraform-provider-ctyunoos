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

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"
  user_names = {
    alpha = "${local.name_prefix}-alpha"
    bravo = "${local.name_prefix}-bravo"
  }
}

resource "ctyun_iam_user" "test" {
  for_each = local.user_names

  name          = each.value
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  name = "${local.name_prefix}-group"
}

resource "ctyun_iam_user_group_membership" "test" {
  for_each = ctyun_iam_user.test

  user  = each.value.name
  group = ctyun_iam_group.test.name
}

resource "ctyun_iam_policy" "test" {
  name = "${local.name_prefix}-users-read"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:ListUsers"]
      Resource = "*"
    }]
  })
}

resource "ctyun_iam_user_policy_attachment" "test" {
  for_each = ctyun_iam_user.test

  user       = each.value.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_users" "test" {
  name_regex = "^${local.name_prefix}-(alpha|bravo)$"

  depends_on = [ctyun_iam_user.test]
}

data "ctyun_iam_user_group_membership" "test" {
  for_each = ctyun_iam_user.test

  user_name = each.value.name

  depends_on = [ctyun_iam_user_group_membership.test]
}

data "ctyun_iam_user_policy_attachments" "test" {
  for_each = ctyun_iam_user.test

  user_name = each.value.name

  depends_on = [ctyun_iam_user_policy_attachment.test]
}

output "attached_policy_arns_by_user" {
  value = {
    for name, attachments in data.ctyun_iam_user_policy_attachments.test :
    name => toset(attachments.attached_policies[*].policy_arn)
  }
}

output "expected_user_names" {
  value = toset(values(local.user_names))
}

output "group_name" {
  value = ctyun_iam_group.test.name
}

output "group_names_by_user" {
  value = {
    for name, membership in data.ctyun_iam_user_group_membership.test :
    name => toset(membership.groups_for_user[*].group_name)
  }
}

output "policy_arn" {
  value = ctyun_iam_policy.test.arn
}

output "returned_user_names" {
  value = data.ctyun_iam_users.test.names
}
