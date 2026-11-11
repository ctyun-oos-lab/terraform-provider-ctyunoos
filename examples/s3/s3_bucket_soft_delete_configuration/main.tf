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

variable "retention_days" {
  type        = number
  default     = 7
  description = "Soft-delete retention period in days. OOS accepts 1 through 15."
  validation {
    condition     = var.retention_days >= 1 && var.retention_days <= 15
    error_message = "retention_days must be between 1 and 15"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-soft-delete-${random_id.suffix.hex}"
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  days   = var.retention_days
}

data "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket     = ctyun_oos_bucket.test.bucket
  depends_on = [ctyun_oos_bucket_soft_delete_configuration.test]
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "resource_id" {
  value = ctyun_oos_bucket_soft_delete_configuration.test.id
}

output "resource_days" {
  value = ctyun_oos_bucket_soft_delete_configuration.test.days
}

output "data_id" {
  value = data.ctyun_oos_bucket_soft_delete_configuration.test.id
}

output "data_days" {
  value = data.ctyun_oos_bucket_soft_delete_configuration.test.days
}
