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
  description = "Existing shared OOS bucket used for the object recover test."
  validation {
    condition     = length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63 && can(regex("^[a-z0-9.-]+$", var.bucket_name))
    error_message = "bucket_name must be 3 to 63 characters using lowercase letters, numbers, periods, and hyphens"
  }
}

variable "create_object" {
  type        = bool
  default     = true
  description = "Whether to manage the source object in this run."
}

variable "recover_object" {
  type        = bool
  default     = false
  description = "Whether to execute the object recovery operation in this run."
}

variable "encoding_type" {
  type        = string
  default     = ""
  description = "Optional URL encoding mode for recovery results."
  validation {
    condition     = var.encoding_type == "" || lower(var.encoding_type) == "url"
    error_message = "encoding_type must be empty or url"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_key = "tf-acc-test/s3-object-recover/${random_id.suffix.hex}/recover.txt"
}

resource "ctyun_oos_object" "test" {
  count   = var.create_object ? 1 : 0
  bucket  = var.bucket_name
  key     = local.object_key
  content = "object recover test"
}

resource "ctyun_oos_object_recover" "test" {
  count         = var.recover_object ? 1 : 0
  bucket        = var.bucket_name
  encoding_type = var.encoding_type == "" ? null : var.encoding_type

  recover {
    key = local.object_key
  }

  depends_on = [ctyun_oos_object.test]
}

output "bucket_name" {
  value = var.bucket_name
}

output "object_key" {
  value = local.object_key
}

output "recover_id" {
  value = var.recover_object ? ctyun_oos_object_recover.test[0].id : ""
}

output "recovered_keys" {
  value = var.recover_object ? ctyun_oos_object_recover.test[0].recovered_keys : []
}

output "recover_errors" {
  value = var.recover_object ? ctyun_oos_object_recover.test[0].errors : []
}
