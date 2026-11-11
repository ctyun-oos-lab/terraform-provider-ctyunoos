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

variable "bucket_name" {
  type        = string
  description = "Existing shared OOS bucket used for the soft-delete object test."
  validation {
    condition     = length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63 && can(regex("^[a-z0-9.-]+$", var.bucket_name))
    error_message = "bucket_name must be 3 to 63 characters using lowercase letters, numbers, periods, and hyphens"
  }
}

variable "create_object" {
  type        = bool
  default     = true
  description = "Whether the test object should be managed in this run."
}

variable "max_keys" {
  type        = number
  default     = 1000
  description = "Maximum number of soft-deleted objects to return."
  validation {
    condition     = var.max_keys >= 1
    error_message = "max_keys must be at least 1"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_key = "tf-acc-test/s3-objects-soft-delete/${random_id.suffix.hex}/soft-delete.txt"
}

resource "ctyun_oos_object" "test" {
  count   = var.create_object ? 1 : 0
  bucket  = var.bucket_name
  key     = local.object_key
  content = "soft-delete test object"
}

data "ctyun_oos_objects_soft_delete" "test" {
  bucket   = var.bucket_name
  max_keys = var.max_keys

  depends_on = [ctyun_oos_object.test]
}

locals {
  matching_contents = [
    for item in data.ctyun_oos_objects_soft_delete.test.contents : item
    if item.key == local.object_key
  ]
}

output "bucket_name" {
  value = var.bucket_name
}

output "object_key" {
  value = local.object_key
}

output "data_id" {
  value = data.ctyun_oos_objects_soft_delete.test.id
}

output "data_max_keys" {
  value = data.ctyun_oos_objects_soft_delete.test.max_keys
}

output "soft_deleted_count" {
  value = length(local.matching_contents)
}

output "soft_deleted_at" {
  value = length(local.matching_contents) == 0 ? "" : local.matching_contents[0].soft_delete_at
}
