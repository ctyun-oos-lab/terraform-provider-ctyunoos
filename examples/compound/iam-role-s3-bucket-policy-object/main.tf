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
  role_name   = "${local.name_prefix}-role"
  bucket_name = "${local.name_prefix}-bucket"
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

resource "ctyun_oos_bucket" "test" {
  bucket        = local.bucket_name
  force_destroy = true

  depends_on = [ctyun_iam_role.test]
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        CTYUN = ctyun_iam_role.test.arn
      }
      Action   = ["oos:GetObject"]
      Resource = "arn:ctyun:oos:::${ctyun_oos_bucket.test.bucket}/*"
    }]
  })

  # The dependency chain is role -> bucket -> policy, so teardown is policy -> bucket -> role.
  depends_on = [ctyun_iam_role.test, ctyun_oos_bucket.test]
}

data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket_policy.test.bucket

  depends_on = [ctyun_oos_bucket_policy.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "policy_bucket" {
  value = data.ctyun_oos_bucket_policy.test.bucket
}

output "role_arn" {
  value = ctyun_iam_role.test.arn
}

output "policy_principal_ctyun" {
  value = jsondecode(data.ctyun_oos_bucket_policy.test.policy).Statement[0].Principal.CTYUN
}
