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

variable "action" {
  type        = string
  default     = "oos:*"
  description = "Action granted by the bucket policy."

  validation {
    condition     = contains(["oos:*", "oos:GetObject", "oos:ListBucket"], var.action)
    error_message = "action must be oos:*, oos:GetObject, or oos:ListBucket"
  }
}

variable "effect" {
  type        = string
  default     = "Allow"
  description = "Policy statement effect."

  validation {
    condition     = contains(["Allow", "Deny"], var.effect)
    error_message = "effect must be Allow or Deny"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-policy-${random_id.suffix.hex}"
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "tf-s3-bucket-policy"
    Statement = [{
      Sid       = "TestBucketStatement"
      Effect    = var.effect
      Principal = "*"
      Action    = [var.action]
      Resource  = "arn:ctyun:oos:::${local.bucket_name}/*"
    }]
  })
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = local.policy
}

locals {
  policy_document = jsondecode(ctyun_oos_bucket_policy.test.policy)
  statement       = one(local.policy_document.Statement)
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "policy_id" {
  value = ctyun_oos_bucket_policy.test.id
}

output "policy_json" {
  value = ctyun_oos_bucket_policy.test.policy
}

output "policy_version" {
  value = local.policy_document.Version
}

output "statement_effect" {
  value = local.statement.Effect
}

output "statement_action" {
  value = one(local.statement.Action)
}

output "statement_resource" {
  value = local.statement.Resource
}
