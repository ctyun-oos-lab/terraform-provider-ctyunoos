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

variable "trail_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated CloudTrail trail name. Used by validation tests."

  validation {
    condition     = var.trail_name == null ? true : (length(var.trail_name) >= 3 && length(var.trail_name) <= 128)
    error_message = "trail_name must be 3 to 128 characters"
  }
}

variable "enable_logging" {
  type        = bool
  default     = true
  description = "Whether the CloudTrail trail should deliver logs."
}

variable "event_selector_enabled" {
  type        = bool
  default     = false
  description = "Whether to declare the optional event_selector block."
}

variable "read_write_type" {
  type        = string
  default     = "All"
  description = "CloudTrail event selector mode."

  validation {
    condition     = contains(["All", "ReadOnly", "WriteOnly"], var.read_write_type)
    error_message = "read_write_type must be All, ReadOnly, or WriteOnly"
  }
}

variable "s3_key_prefix" {
  type        = string
  default     = null
  nullable    = true
  description = "Optional prefix for CloudTrail log objects."

  validation {
    condition     = var.s3_key_prefix == null ? true : length(var.s3_key_prefix) <= 200
    error_message = "s3_key_prefix must be at most 200 characters"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-cli-${random_id.suffix.hex}"
  bucket_name = "${local.name_prefix}-bucket"
  trail_name  = coalesce(var.trail_name, "${local.name_prefix}-trail")
}

resource "ctyun_oos_bucket" "trail_logs" {
  bucket        = local.bucket_name
  force_destroy = true
}

resource "ctyun_cloudtrail" "test" {
  name           = local.trail_name
  s3_bucket_name = ctyun_oos_bucket.trail_logs.bucket
  s3_key_prefix  = var.s3_key_prefix
  enable_logging = var.enable_logging

  dynamic "event_selector" {
    for_each = var.event_selector_enabled ? [var.read_write_type] : []

    content {
      read_write_type = event_selector.value
    }
  }

  depends_on = [ctyun_oos_bucket.trail_logs]
}

data "ctyun_cloudtrail" "test" {
  name       = ctyun_cloudtrail.test.name
  depends_on = [ctyun_cloudtrail.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.trail_logs.bucket
}

output "trail_id" {
  value = ctyun_cloudtrail.test.id
}

output "trail_arn" {
  value = ctyun_cloudtrail.test.arn
}

output "trail_name" {
  value = ctyun_cloudtrail.test.name
}

output "trail_bucket_name" {
  value = ctyun_cloudtrail.test.s3_bucket_name
}

output "trail_key_prefix" {
  value = ctyun_cloudtrail.test.s3_key_prefix
}

output "trail_enable_logging" {
  value = ctyun_cloudtrail.test.enable_logging
}

output "trail_event_selector_count" {
  value = length(ctyun_cloudtrail.test.event_selector)
}

output "trail_read_write_type" {
  value = try(ctyun_cloudtrail.test.event_selector[0].read_write_type, "")
}

output "data_trail_id" {
  value = data.ctyun_cloudtrail.test.id
}

output "data_trail_arn" {
  value = data.ctyun_cloudtrail.test.arn
}

output "data_trail_name" {
  value = data.ctyun_cloudtrail.test.name
}

output "data_trail_bucket_name" {
  value = data.ctyun_cloudtrail.test.s3_bucket_name
}

output "data_trail_key_prefix" {
  value = data.ctyun_cloudtrail.test.s3_key_prefix
}

output "data_trail_enable_logging" {
  value = data.ctyun_cloudtrail.test.enable_logging
}

output "data_trail_event_selector_count" {
  value = length(data.ctyun_cloudtrail.test.event_selector)
}

output "data_trail_read_write_type" {
  value = try(data.ctyun_cloudtrail.test.event_selector[0].read_write_type, "")
}

output "data_start_logging_time" {
  value = data.ctyun_cloudtrail.test.start_logging_time
}

output "data_stop_logging_time" {
  value = data.ctyun_cloudtrail.test.stop_logging_time
}

output "data_latest_delivery_error" {
  value = data.ctyun_cloudtrail.test.latest_delivery_error
}

output "data_latest_delivery_time" {
  value = data.ctyun_cloudtrail.test.latest_delivery_time
}
