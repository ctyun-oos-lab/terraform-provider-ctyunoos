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
  log_bucket  = "${local.name_prefix}-log"
}

resource "ctyun_oos_bucket" "log" {
  bucket        = local.log_bucket
  force_destroy = true
}

resource "ctyun_oos_bucket" "main" {
  bucket        = local.bucket_name
  force_destroy = true

  logging {
    target_bucket = ctyun_oos_bucket.log.bucket
    target_prefix = "access-logs/"
  }

  lifecycle_rule {
    id      = "expire-composite-objects"
    enabled = true
    prefix  = ""

    expiration {
      days = 30
    }
  }

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "ctyun_oos_bucket_policy" "main" {
  bucket = ctyun_oos_bucket.main.bucket

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

output "bucket_name" {
  value = ctyun_oos_bucket.main.bucket
}

output "inline_encryption_algorithm" {
  value = ctyun_oos_bucket.main.server_side_encryption_configuration[0].rule[0].apply_server_side_encryption_by_default[0].sse_algorithm
}

output "inline_lifecycle_days" {
  value = ctyun_oos_bucket.main.lifecycle_rule[0].expiration[0].days
}

output "bucket_policy_bucket" {
  value = ctyun_oos_bucket_policy.main.bucket
}

output "inline_logging_target_bucket" {
  value = ctyun_oos_bucket.main.logging[0].target_bucket
}

output "log_bucket_name" {
  value = ctyun_oos_bucket.log.bucket
}
