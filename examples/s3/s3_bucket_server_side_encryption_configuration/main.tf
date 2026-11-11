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

variable "sse_algorithm" {
  type        = string
  default     = "AES256"
  description = "Default server-side encryption algorithm. OOS supports AES256 only."

  validation {
    condition     = var.sse_algorithm == "AES256"
    error_message = "sse_algorithm must be AES256"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-sse-${random_id.suffix.hex}"
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = var.sse_algorithm
    }
  }
}

data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_server_side_encryption_configuration.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "resource_id" {
  value = ctyun_oos_bucket_server_side_encryption_configuration.test.id
}

output "resource_rule_count" {
  value = length(ctyun_oos_bucket_server_side_encryption_configuration.test.rule)
}

output "resource_sse_algorithm" {
  value = ctyun_oos_bucket_server_side_encryption_configuration.test.rule[0].apply_server_side_encryption_by_default[0].sse_algorithm
}

output "data_id" {
  value = data.ctyun_oos_bucket_server_side_encryption_configuration.test.id
}

output "data_rule_count" {
  value = length(data.ctyun_oos_bucket_server_side_encryption_configuration.test.rule)
}

output "data_sse_algorithm" {
  value = data.ctyun_oos_bucket_server_side_encryption_configuration.test.rule[0].apply_server_side_encryption_by_default[0].sse_algorithm
}
