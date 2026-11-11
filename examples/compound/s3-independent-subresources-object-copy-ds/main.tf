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
  bucket_name = "${local.name_prefix}-main"
}

resource "ctyun_oos_bucket" "test" {
  bucket        = local.bucket_name
  force_destroy = true
}

resource "ctyun_oos_bucket_acl" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  acl    = "private"
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = "*"
      Action    = ["oos:GetObject"]
      Resource  = "arn:ctyun:oos:::${local.bucket_name}/*"
    }]
  })
}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket_policy.test.bucket

  depends_on = [ctyun_oos_bucket_policy.test]
}

data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket_server_side_encryption_configuration.test.bucket

  depends_on = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "acl_bucket" {
  value = ctyun_oos_bucket_acl.test.bucket
}

output "acl_value" {
  value = ctyun_oos_bucket_acl.test.acl
}

output "policy_bucket" {
  value = data.ctyun_oos_bucket_policy.test.bucket
}

output "policy_document" {
  value = data.ctyun_oos_bucket_policy.test.policy
}

output "policy_resource" {
  value = jsondecode(data.ctyun_oos_bucket_policy.test.policy).Statement[0].Resource
}

output "encryption_bucket" {
  value = data.ctyun_oos_bucket_server_side_encryption_configuration.test.bucket
}

output "encryption_algorithm" {
  value = data.ctyun_oos_bucket_server_side_encryption_configuration.test.rule[0].apply_server_side_encryption_by_default[0].sse_algorithm
}
