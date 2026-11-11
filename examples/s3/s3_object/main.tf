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
  description = "Existing shared OOS bucket used for the object test."
  validation {
    condition     = length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63 && can(regex("^[a-z0-9.-]+$", var.bucket_name))
    error_message = "bucket_name must be 3 to 63 characters using lowercase letters, numbers, periods, and hyphens"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/s3-object/${random_id.suffix.hex}"
}

variable "content" {
  type        = string
  default     = "object content v1"
  description = "Inline object content."
}

variable "storage_class" {
  type        = string
  default     = "STANDARD"
  description = "Object storage class."
  validation {
    condition     = contains(["STANDARD"], var.storage_class)
    error_message = "storage_class must be STANDARD"
  }
}

resource "ctyun_oos_object" "test" {
  bucket        = var.bucket_name
  key           = "${local.object_prefix}/inline.txt"
  content       = var.content
  content_type  = "text/plain"
  storage_class = var.storage_class

  metadata = {
    project = "terraform-test"
    owner   = "codex"
  }
}

resource "ctyun_oos_object" "source" {
  bucket = var.bucket_name
  key    = "${local.object_prefix}/source.txt"
  source = "${path.module}/source.txt"

  cache_control       = "max-age=60"
  content_disposition = "attachment; filename=source.txt"
  content_encoding    = "identity"
  content_md5         = "6m7OQ6rllrMYURIky1gesg=="
  content_type        = "text/plain"

  forbid_overwrite = true
  limit            = "rate=1024"

  metadata = {
    project = "terraform-test"
    owner   = "codex-source"
  }
}

output "object_id" {
  value = ctyun_oos_object.test.id
}

output "bucket_name" {
  value = var.bucket_name
}

output "object_arn" {
  value = ctyun_oos_object.test.arn
}

output "object_bucket" {
  value = ctyun_oos_object.test.bucket
}

output "object_key" {
  value = ctyun_oos_object.test.key
}

output "object_etag" {
  value = ctyun_oos_object.test.etag
}

output "object_content_type" {
  value = ctyun_oos_object.test.content_type
}

output "object_storage_class" {
  value = ctyun_oos_object.test.storage_class
}

output "object_metadata" {
  value = ctyun_oos_object.test.metadata
}

output "object_cache_control" {
  value = ctyun_oos_object.test.cache_control
}

output "object_content_disposition" {
  value = ctyun_oos_object.test.content_disposition
}

output "object_content_encoding" {
  value = ctyun_oos_object.test.content_encoding
}

output "object_content_md5" {
  value = ctyun_oos_object.test.content_md5
}

output "object_data_location" {
  value = ctyun_oos_object.test.data_location
}

output "object_metadata_location" {
  value = ctyun_oos_object.test.metadata_location
}

output "object_server_side_encryption" {
  value = ctyun_oos_object.test.server_side_encryption
}

output "object_forbid_overwrite" {
  value = ctyun_oos_object.test.forbid_overwrite
}

output "object_limit" {
  value = ctyun_oos_object.test.limit
}

output "object_source" {
  value = ctyun_oos_object.test.source
}

output "source_object_id" {
  value = ctyun_oos_object.source.id
}

output "source_object_arn" {
  value = ctyun_oos_object.source.arn
}

output "source_object_bucket" {
  value = ctyun_oos_object.source.bucket
}

output "source_object_key" {
  value = ctyun_oos_object.source.key
}

output "source_object_cache_control" {
  value = ctyun_oos_object.source.cache_control
}

output "source_object_content_disposition" {
  value = ctyun_oos_object.source.content_disposition
}

output "source_object_content_encoding" {
  value = ctyun_oos_object.source.content_encoding
}

output "source_object_content_md5" {
  value = ctyun_oos_object.source.content_md5
}

output "source_object_content_type" {
  value = ctyun_oos_object.source.content_type
}

output "source_object_data_location" {
  value = ctyun_oos_object.source.data_location
}

output "source_object_etag" {
  value = ctyun_oos_object.source.etag
}

output "source_object_metadata" {
  value = ctyun_oos_object.source.metadata
}

output "source_object_metadata_location" {
  value = ctyun_oos_object.source.metadata_location
}

output "source_object_server_side_encryption" {
  value = ctyun_oos_object.source.server_side_encryption
}

output "source_object_storage_class" {
  value = ctyun_oos_object.source.storage_class
}

output "source_object_forbid_overwrite" {
  value = ctyun_oos_object.source.forbid_overwrite
}

output "source_object_limit" {
  value = ctyun_oos_object.source.limit
}

output "source_object_source" {
  value = ctyun_oos_object.source.source
}
