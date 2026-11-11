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
  trail_name  = "${local.name_prefix}-trail"
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

  # The dependency chain is role -> bucket -> policy -> trail.
  depends_on = [ctyun_iam_role.test, ctyun_oos_bucket.test]
}

resource "ctyun_cloudtrail" "test" {
  name           = local.trail_name
  s3_bucket_name = ctyun_oos_bucket.test.bucket

  # enable_logging is intentionally omitted to verify its provider default: true.
  depends_on = [ctyun_oos_bucket_policy.test]
}

data "ctyun_cloudtrail" "test" {
  name = ctyun_cloudtrail.test.name

  depends_on = [ctyun_cloudtrail.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "role_arn" {
  value = ctyun_iam_role.test.arn
}

output "policy_principal_ctyun" {
  value = jsondecode(ctyun_oos_bucket_policy.test.policy).Statement[0].Principal.CTYUN
}

output "trail_bucket_name" {
  value = data.ctyun_cloudtrail.test.s3_bucket_name
}

output "trail_logging_enabled" {
  value = data.ctyun_cloudtrail.test.enable_logging
}
