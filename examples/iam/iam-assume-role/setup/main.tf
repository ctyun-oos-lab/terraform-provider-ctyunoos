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

# Root credentials from CTYUNOOS_* — used only to create the IAM fixture.
# OOS IAM rejects AssumeRole from root accounts; the assumed modules must use
# the source IAM user's access key created below.
provider "ctyun" {}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"

  account_summary_only = {
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:GetAccountSummary"]
      Resource = "*"
    }]
  }

  assume_role_b_only = {
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:AssumeRole"]
      Resource = ctyun_iam_role.chain_b.arn
    }]
  }
}

# Source IAM user (non-root). Only this identity may call AssumeRole.
resource "ctyun_iam_user" "source" {
  name          = "${local.name_prefix}-ar-source"
  force_destroy = true
}

resource "ctyun_iam_access_key" "source" {
  user = ctyun_iam_user.source.name
}

# Single-hop target: trusts the source user; allows GetAccountSummary only.
resource "ctyun_iam_role" "target" {
  name = "${local.name_prefix}-ar-target"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = ["iam:AssumeRole"]
      Principal = { CTYUN = [ctyun_iam_user.source.arn] }
    }]
  })
}

resource "ctyun_iam_policy" "target" {
  name   = "${local.name_prefix}-ar-target-policy"
  policy = jsonencode(local.account_summary_only)
}

resource "ctyun_iam_role_policy_attachment" "target" {
  role       = ctyun_iam_role.target.name
  policy_arn = ctyun_iam_policy.target.arn
}

# Chain hop A: trusts source user; may only AssumeRole into chain B.
resource "ctyun_iam_role" "chain_a" {
  name = "${local.name_prefix}-ar-chain-a"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = ["iam:AssumeRole"]
      Principal = { CTYUN = [ctyun_iam_user.source.arn] }
    }]
  })
}

resource "ctyun_iam_policy" "chain_a" {
  name   = "${local.name_prefix}-ar-chain-a-policy"
  policy = jsonencode(local.assume_role_b_only)
}

resource "ctyun_iam_role_policy_attachment" "chain_a" {
  role       = ctyun_iam_role.chain_a.name
  policy_arn = ctyun_iam_policy.chain_a.arn
}

# Chain hop B: trusts only role A; GetAccountSummary only.
resource "ctyun_iam_role" "chain_b" {
  name = "${local.name_prefix}-ar-chain-b"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = ["iam:AssumeRole"]
      Principal = { CTYUN = [ctyun_iam_role.chain_a.arn] }
    }]
  })
}

resource "ctyun_iam_policy" "chain_b" {
  name   = "${local.name_prefix}-ar-chain-b-policy"
  policy = jsonencode(local.account_summary_only)
}

resource "ctyun_iam_role_policy_attachment" "chain_b" {
  role       = ctyun_iam_role.chain_b.name
  policy_arn = ctyun_iam_policy.chain_b.arn
}

# Source user may AssumeRole into the single-hop target and chain A only.
resource "ctyun_iam_policy" "source_assume" {
  name = "${local.name_prefix}-ar-source-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["iam:AssumeRole"]
      Resource = [
        ctyun_iam_role.target.arn,
        ctyun_iam_role.chain_a.arn,
      ]
    }]
  })
}

resource "ctyun_iam_user_policy_attachment" "source" {
  user       = ctyun_iam_user.source.name
  policy_arn = ctyun_iam_policy.source_assume.arn
}

output "source_user_name" {
  value = ctyun_iam_user.source.name
}

output "source_user_arn" {
  value = ctyun_iam_user.source.arn
}

output "source_access_key" {
  value = ctyun_iam_access_key.source.id
}

output "source_secret_key" {
  value     = ctyun_iam_access_key.source.secret
  sensitive = true
}

output "role_arn" {
  value = ctyun_iam_role.target.arn
}

output "role_name" {
  value = ctyun_iam_role.target.name
}

output "role_a_arn" {
  value = ctyun_iam_role.chain_a.arn
}

output "role_b_arn" {
  value = ctyun_iam_role.chain_b.arn
}

output "attachment_target_id" {
  value = ctyun_iam_role_policy_attachment.target.id
}

output "attachment_chain_a_id" {
  value = ctyun_iam_role_policy_attachment.chain_a.id
}

output "attachment_chain_b_id" {
  value = ctyun_iam_role_policy_attachment.chain_b.id
}

output "source_attachment_id" {
  value = ctyun_iam_user_policy_attachment.source.id
}
