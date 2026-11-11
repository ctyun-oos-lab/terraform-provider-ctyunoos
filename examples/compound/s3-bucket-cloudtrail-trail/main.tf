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
  bucket_name = "${local.name_prefix}-bucket"
  trail_name  = "${local.name_prefix}-trail"
}

resource "ctyun_oos_bucket" "test" {
  bucket        = local.bucket_name
  force_destroy = true
}

resource "ctyun_cloudtrail" "test" {
  name           = local.trail_name
  s3_bucket_name = ctyun_oos_bucket.test.bucket

  # enable_logging is intentionally omitted to verify its provider default: true.
}

data "ctyun_cloudtrail" "test" {
  name = ctyun_cloudtrail.test.name

  depends_on = [ctyun_cloudtrail.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "trail_bucket_name" {
  value = data.ctyun_cloudtrail.test.s3_bucket_name
}

output "trail_logging_enabled" {
  value = data.ctyun_cloudtrail.test.enable_logging
}
