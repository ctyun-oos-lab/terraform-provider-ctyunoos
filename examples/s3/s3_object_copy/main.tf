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
  description = "Existing shared OOS bucket used for both source and target objects."
  validation {
    condition     = length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63 && can(regex("^[a-z0-9.-]+$", var.bucket_name))
    error_message = "bucket_name must be 3 to 63 characters using lowercase letters, numbers, periods, and hyphens"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/s3-object-copy/${random_id.suffix.hex}"
}

variable "target_storage_class" {
  type        = string
  default     = "STANDARD"
  description = "Storage class assigned to the copied object."
  validation {
    condition     = contains(["STANDARD"], var.target_storage_class)
    error_message = "target_storage_class must be STANDARD"
  }
}

variable "target_owner" {
  type        = string
  default     = "team-a"
  description = "Owner metadata assigned to the copied object."
}

resource "ctyun_oos_object" "source" {
  bucket  = var.bucket_name
  key     = "${local.object_prefix}/source.txt"
  content = "source object for copy test"
}

resource "ctyun_oos_object_copy" "test" {
  bucket = var.bucket_name
  key    = "${local.object_prefix}/target.txt"
  source = "${ctyun_oos_object.source.bucket}/${ctyun_oos_object.source.key}"

  content_type       = "text/plain"
  metadata_directive = "REPLACE"
  storage_class      = var.target_storage_class
  metadata = {
    project = "terraform-test"
    owner   = var.target_owner
  }

  depends_on = [ctyun_oos_object.source]
}

resource "ctyun_oos_object_copy" "headers" {
  bucket = var.bucket_name
  key    = "${local.object_prefix}/target-headers.txt"
  source = "${ctyun_oos_object.source.bucket}/${ctyun_oos_object.source.key}"

  cache_control            = "max-age=60"
  content_disposition      = "attachment; filename=copy-target-headers.txt"
  content_encoding         = "identity"
  content_type             = "text/plain"
  copy_if_match            = ctyun_oos_object.source.etag
  copy_if_modified_since   = "2000-01-01T00:00:00Z"
  copy_if_none_match       = "non-matching-etag"
  copy_if_unmodified_since = "2100-01-01T00:00:00Z"
  metadata_directive       = "REPLACE"

  metadata = {
    source = "conditional-copy"
  }

  depends_on = [ctyun_oos_object.source]
}

output "source_id" {
  value = ctyun_oos_object.source.id
}

output "bucket_name" {
  value = var.bucket_name
}

output "source_bucket" {
  value = ctyun_oos_object.source.bucket
}

output "source_key" {
  value = ctyun_oos_object.source.key
}

output "copy_id" {
  value = ctyun_oos_object_copy.test.id
}

output "copy_arn" {
  value = ctyun_oos_object_copy.test.arn
}

output "copy_bucket" {
  value = ctyun_oos_object_copy.test.bucket
}

output "copy_key" {
  value = ctyun_oos_object_copy.test.key
}

output "copy_source" {
  value = ctyun_oos_object_copy.test.source
}

output "copy_etag" {
  value = ctyun_oos_object_copy.test.etag
}

output "copy_content_type" {
  value = ctyun_oos_object_copy.test.content_type
}

output "copy_storage_class" {
  value = ctyun_oos_object_copy.test.storage_class
}

output "copy_metadata" {
  value = ctyun_oos_object_copy.test.metadata
}

output "copy_cache_control" {
  value = ctyun_oos_object_copy.test.cache_control
}

output "copy_content_disposition" {
  value = ctyun_oos_object_copy.test.content_disposition
}

output "copy_content_encoding" {
  value = ctyun_oos_object_copy.test.content_encoding
}

output "copy_data_location" {
  value = ctyun_oos_object_copy.test.data_location
}

output "copy_expiration" {
  value = ctyun_oos_object_copy.test.expiration
}

output "copy_last_modified" {
  value = ctyun_oos_object_copy.test.last_modified
}

output "copy_metadata_location" {
  value = ctyun_oos_object_copy.test.metadata_location
}

output "copy_server_side_encryption" {
  value = ctyun_oos_object_copy.test.server_side_encryption
}

output "copy_metadata_directive" {
  value = ctyun_oos_object_copy.test.metadata_directive
}

output "headers_copy_id" {
  value = ctyun_oos_object_copy.headers.id
}

output "headers_copy_arn" {
  value = ctyun_oos_object_copy.headers.arn
}

output "headers_copy_bucket" {
  value = ctyun_oos_object_copy.headers.bucket
}

output "headers_copy_key" {
  value = ctyun_oos_object_copy.headers.key
}

output "headers_copy_source" {
  value = ctyun_oos_object_copy.headers.source
}

output "headers_copy_cache_control" {
  value = ctyun_oos_object_copy.headers.cache_control
}

output "headers_copy_content_disposition" {
  value = ctyun_oos_object_copy.headers.content_disposition
}

output "headers_copy_content_encoding" {
  value = ctyun_oos_object_copy.headers.content_encoding
}

output "headers_copy_content_type" {
  value = ctyun_oos_object_copy.headers.content_type
}

output "headers_copy_copy_if_match" {
  value = ctyun_oos_object_copy.headers.copy_if_match
}

output "headers_copy_copy_if_modified_since" {
  value = ctyun_oos_object_copy.headers.copy_if_modified_since
}

output "headers_copy_copy_if_none_match" {
  value = ctyun_oos_object_copy.headers.copy_if_none_match
}

output "headers_copy_copy_if_unmodified_since" {
  value = ctyun_oos_object_copy.headers.copy_if_unmodified_since
}

output "headers_copy_data_location" {
  value = ctyun_oos_object_copy.headers.data_location
}

output "headers_copy_etag" {
  value = ctyun_oos_object_copy.headers.etag
}

output "headers_copy_expiration" {
  value = ctyun_oos_object_copy.headers.expiration
}

output "headers_copy_last_modified" {
  value = ctyun_oos_object_copy.headers.last_modified
}

output "headers_copy_metadata" {
  value = ctyun_oos_object_copy.headers.metadata
}

output "headers_copy_metadata_directive" {
  value = ctyun_oos_object_copy.headers.metadata_directive
}

output "headers_copy_metadata_location" {
  value = ctyun_oos_object_copy.headers.metadata_location
}

output "headers_copy_server_side_encryption" {
  value = ctyun_oos_object_copy.headers.server_side_encryption
}

output "headers_copy_storage_class" {
  value = ctyun_oos_object_copy.headers.storage_class
}
