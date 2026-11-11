# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}

provider "ctyun" {}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-acl-${random_id.suffix.hex}"
}

variable "acl" {
  type        = string
  default     = "private"
  description = "Canned bucket ACL."
  validation {
    condition     = contains(["private", "public-read", "public-read-write"], var.acl)
    error_message = "acl must be private, public-read, or public-read-write"
  }
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_acl" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  acl    = var.acl
}

output "bucket_created" {
  value = ctyun_oos_bucket.test.bucket
}

output "bucket" {
  value = ctyun_oos_bucket_acl.test.bucket
}

output "acl" {
  value = ctyun_oos_bucket_acl.test.acl
}

output "access_control_policy_count" {
  value = length(ctyun_oos_bucket_acl.test.access_control_policy)
}

output "owner_count" {
  value = length(ctyun_oos_bucket_acl.test.access_control_policy[0].owner)
}

output "grant_count" {
  value = length(ctyun_oos_bucket_acl.test.access_control_policy[0].grant)
}

output "grant_permission" {
  value = ctyun_oos_bucket_acl.test.access_control_policy[0].grant[0].permission
}
