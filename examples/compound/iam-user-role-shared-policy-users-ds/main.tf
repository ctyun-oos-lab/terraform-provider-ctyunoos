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
  user_name   = "${local.name_prefix}-user"
  role_name   = "${local.name_prefix}-role"
  policy_name = "${local.name_prefix}-shared"
}

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_role" "test" {
  name = local.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
      Effect = "Allow"
    }]
  })
}

resource "ctyun_iam_policy" "shared" {
  name = local.policy_name

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
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.shared.arn
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.shared.arn
}

data "ctyun_iam_users" "test" {
  name_regex = "^${local.user_name}$"

  depends_on = [ctyun_iam_user.test]
}

data "ctyun_iam_user_policy_attachments" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_user_policy_attachment.test]
}

data "ctyun_iam_role_policy_attachments" "test" {
  role_name = ctyun_iam_role.test.name

  depends_on = [ctyun_iam_role_policy_attachment.test]
}

output "expected_user_names" {
  value = toset([local.user_name])
}

output "policy_arn" {
  value = ctyun_iam_policy.shared.arn
}

output "returned_user_names" {
  value = data.ctyun_iam_users.test.names
}

output "role_attached_policy_arns" {
  value = toset(data.ctyun_iam_role_policy_attachments.test.attached_policies[*].policy_arn)
}

output "user_attached_policy_arns" {
  value = toset(data.ctyun_iam_user_policy_attachments.test.attached_policies[*].policy_arn)
}
