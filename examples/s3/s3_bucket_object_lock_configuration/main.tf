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

variable "object_lock_enabled" {
  type        = string
  default     = "Disabled"
  description = "Object Lock status. Valid values are Enabled and Disabled."

  validation {
    condition     = contains(["Enabled", "Disabled"], var.object_lock_enabled)
    error_message = "object_lock_enabled must be Enabled or Disabled"
  }
}

variable "retention_days" {
  type        = number
  default     = 30
  description = "Default COMPLIANCE retention period in days."

  validation {
    condition     = var.retention_days >= 1 && var.retention_days <= 36500
    error_message = "retention_days must be between 1 and 36500"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-object-lock-${random_id.suffix.hex}"
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket              = ctyun_oos_bucket.test.bucket
  object_lock_enabled = var.object_lock_enabled

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = var.retention_days
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_object_lock_configuration.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "resource_id" {
  value = ctyun_oos_bucket_object_lock_configuration.test.id
}

output "resource_object_lock_enabled" {
  value = ctyun_oos_bucket_object_lock_configuration.test.object_lock_enabled
}

output "resource_rule_count" {
  value = length(ctyun_oos_bucket_object_lock_configuration.test.rule)
}

output "resource_retention_mode" {
  value = ctyun_oos_bucket_object_lock_configuration.test.rule[0].default_retention[0].mode
}

output "resource_retention_days" {
  value = ctyun_oos_bucket_object_lock_configuration.test.rule[0].default_retention[0].days
}

output "data_id" {
  value = data.ctyun_oos_bucket_object_lock_configuration.test.id
}

output "data_object_lock_enabled" {
  value = data.ctyun_oos_bucket_object_lock_configuration.test.object_lock_enabled
}

output "data_rule_count" {
  value = length(data.ctyun_oos_bucket_object_lock_configuration.test.rule)
}

output "data_retention_mode" {
  value = data.ctyun_oos_bucket_object_lock_configuration.test.rule[0].default_retention[0].mode
}

output "data_retention_days" {
  value = data.ctyun_oos_bucket_object_lock_configuration.test.rule[0].default_retention[0].days
}
