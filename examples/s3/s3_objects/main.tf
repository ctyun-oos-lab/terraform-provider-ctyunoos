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
  description = "Existing shared OOS bucket used for the object-listing test. It is not destroyed by the suite."
  validation {
    condition     = length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63 && can(regex("^[a-z0-9.-]+$", var.bucket_name))
    error_message = "bucket_name must be 3 to 63 characters using lowercase letters, numbers, periods, and hyphens"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/s3-objects/${random_id.suffix.hex}"
  data_prefix   = "${local.object_prefix}/${var.query_prefix}"
  object_keys = [
    "${local.object_prefix}/objects/prefix1/a.txt",
    "${local.object_prefix}/objects/prefix1/b.txt",
    "${local.object_prefix}/objects/prefix2/c.txt",
  ]
}

variable "query_prefix" {
  type        = string
  default     = "objects/"
  description = "Prefix used to list test objects."
  validation {
    condition     = contains(["objects/", "objects/prefix1/"], var.query_prefix)
    error_message = "query_prefix must be objects/ or objects/prefix1/"
  }
}

variable "delimiter" {
  type        = string
  default     = ""
  description = "Optional object listing delimiter. OOS supports /."
  validation {
    condition     = var.delimiter == "" || var.delimiter == "/"
    error_message = "delimiter must be empty or /"
  }
}

variable "max_keys" {
  type        = number
  default     = 1000
  description = "Maximum number of object keys returned."
  validation {
    condition     = var.max_keys >= 1
    error_message = "max_keys must be at least 1"
  }
}

resource "ctyun_oos_object" "prefix1_a" {
  bucket  = var.bucket_name
  key     = local.object_keys[0]
  content = "object-a"
}

resource "ctyun_oos_object" "prefix1_b" {
  bucket  = var.bucket_name
  key     = local.object_keys[1]
  content = "object-b"
}

resource "ctyun_oos_object" "prefix2" {
  bucket  = var.bucket_name
  key     = local.object_keys[2]
  content = "object-c"
}

data "ctyun_oos_objects" "test" {
  bucket     = var.bucket_name
  prefix     = local.data_prefix
  delimiter  = var.delimiter == "" ? null : var.delimiter
  max_keys   = var.max_keys
  depends_on = [ctyun_oos_object.prefix1_a, ctyun_oos_object.prefix1_b, ctyun_oos_object.prefix2]
}

output "bucket_name" {
  value = var.bucket_name
}

output "object_prefix" {
  value = local.object_prefix
}

output "expected_keys" {
  value = local.object_keys
}

output "data_id" {
  value = data.ctyun_oos_objects.test.id
}

output "data_prefix" {
  value = data.ctyun_oos_objects.test.prefix
}

output "data_delimiter" {
  value = data.ctyun_oos_objects.test.delimiter
}

output "data_max_keys" {
  value = data.ctyun_oos_objects.test.max_keys
}

output "keys" {
  value = data.ctyun_oos_objects.test.keys
}

output "common_prefixes" {
  value = data.ctyun_oos_objects.test.common_prefix
}
