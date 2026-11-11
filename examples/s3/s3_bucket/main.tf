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

variable "acl" {
  type        = string
  default     = "private"
  description = "Bucket ACL."
  validation {
    condition     = contains(["private", "public-read", "public-read-write"], var.acl)
    error_message = "acl must be private, public-read, or public-read-write"
  }
}

variable "policy_action" {
  type    = string
  default = "oos:GetObject"
  validation {
    condition     = contains(["oos:*", "oos:GetObject", "oos:ListBucket"], var.policy_action)
    error_message = "policy_action must be oos:*, oos:GetObject, or oos:ListBucket"
  }
}

variable "policy_effect" {
  type    = string
  default = "Allow"
  validation {
    condition     = contains(["Allow", "Deny"], var.policy_effect)
    error_message = "policy_effect must be Allow or Deny"
  }
}

variable "object_lock_enabled" {
  type    = string
  default = "Disabled"
  validation {
    condition     = contains(["Enabled", "Disabled"], var.object_lock_enabled)
    error_message = "object_lock_enabled must be Enabled or Disabled"
  }
}

variable "object_lock_days" {
  type    = number
  default = 30
  validation {
    condition     = var.object_lock_days >= 1 && var.object_lock_days <= 36500
    error_message = "object_lock_days must be between 1 and 36500"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-bucket-${random_id.suffix.hex}"
  bucket_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "InlineBucketStatement"
      Effect    = var.policy_effect
      Principal = "*"
      Action    = [var.policy_action]
      Resource  = var.policy_action == "oos:ListBucket" ? "arn:ctyun:oos:::${local.bucket_name}" : "arn:ctyun:oos:::${local.bucket_name}/*"
    }]
  })
}

resource "ctyun_oos_bucket" "test" {
  bucket        = local.bucket_name
  acl           = var.acl
  force_destroy = true

  lifecycle_rule {
    id      = "inline-lifecycle"
    enabled = true
    prefix  = "terraform-tests/"
    expiration {
      days = 30
    }
  }

  logging {
    target_bucket = ctyun_oos_bucket.logging_target.bucket
    target_prefix = "access-logs/"
  }

  object_lock_configuration {
    object_lock_enabled = var.object_lock_enabled
    rule {
      default_retention {
        mode = "COMPLIANCE"
        days = var.object_lock_days
      }
    }
  }

  policy = local.bucket_policy

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "ctyun_oos_bucket" "logging_target" {
  bucket        = "tf-s3-bucket-log-${random_id.suffix.hex}"
  force_destroy = true
}

output "bucket" {
  value = ctyun_oos_bucket.test.bucket
}

output "bucket_id" {
  value = ctyun_oos_bucket.test.id
}

output "bucket_arn" {
  value = ctyun_oos_bucket.test.arn
}

output "bucket_acl" {
  value = ctyun_oos_bucket.test.acl
}

output "bucket_force_destroy" {
  value = ctyun_oos_bucket.test.force_destroy
}

output "bucket_metadata_location" {
  value = try(ctyun_oos_bucket.test.metadata_location_constraint[0].location, "")
}

output "bucket_data_location_type" {
  value = try(ctyun_oos_bucket.test.data_location_constraint[0].type, "")
}

output "bucket_lifecycle_rule_count" {
  value = length(ctyun_oos_bucket.test.lifecycle_rule)
}

output "bucket_logging_target" {
  value = try(ctyun_oos_bucket.test.logging[0].target_bucket, "")
}

output "bucket_object_lock_enabled" {
  value = try(ctyun_oos_bucket.test.object_lock_configuration[0].object_lock_enabled, "")
}

output "bucket_policy" {
  value = ctyun_oos_bucket.test.policy
}

output "bucket_policy_action" {
  value = var.policy_action
}

output "bucket_policy_effect" {
  value = var.policy_effect
}

output "bucket_object_lock_days" {
  value = try(ctyun_oos_bucket.test.object_lock_configuration[0].rule[0].default_retention[0].days, 0)
}

output "bucket_sse_algorithm" {
  value = try(ctyun_oos_bucket.test.server_side_encryption_configuration[0].rule[0].apply_server_side_encryption_by_default[0].sse_algorithm, "")
}
