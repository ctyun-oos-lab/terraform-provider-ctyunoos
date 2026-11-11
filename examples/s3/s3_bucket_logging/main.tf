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
      version = "~> 3.6"
    }
  }
}

provider "ctyun" {}

variable "target_prefix" {
  type        = string
  default     = "access-logs/"
  description = "Prefix for delivered access log objects."
  validation {
    condition     = length(var.target_prefix) <= 1024 && can(regex("/$", var.target_prefix))
    error_message = "target_prefix must be at most 1024 characters and end with a slash"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  source_bucket = "tf-s3-logging-${random_id.suffix.hex}"
  target_bucket = "tf-s3-log-target-${random_id.suffix.hex}"
}

resource "ctyun_oos_bucket" "source" {
  bucket        = local.source_bucket
  force_destroy = true
}

resource "ctyun_oos_bucket" "target" {
  bucket        = local.target_bucket
  force_destroy = true
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket        = ctyun_oos_bucket.source.bucket
  target_bucket = ctyun_oos_bucket.target.bucket
  target_prefix = var.target_prefix
  depends_on    = [ctyun_oos_bucket.source, ctyun_oos_bucket.target]
}

output "source_bucket" {
  value = ctyun_oos_bucket.source.bucket
}

output "target_bucket" {
  value = ctyun_oos_bucket.target.bucket
}

output "logging_id" {
  value = ctyun_oos_bucket_logging.test.id
}

output "logging_bucket" {
  value = ctyun_oos_bucket_logging.test.bucket
}

output "logging_target_bucket" {
  value = ctyun_oos_bucket_logging.test.target_bucket
}

output "logging_target_prefix" {
  value = ctyun_oos_bucket_logging.test.target_prefix
}
