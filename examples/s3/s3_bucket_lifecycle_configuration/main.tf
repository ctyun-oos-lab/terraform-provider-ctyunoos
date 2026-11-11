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

variable "status" {
  type        = string
  default     = "Enabled"
  description = "Lifecycle rule status. Valid values are Enabled and Disabled."

  validation {
    condition     = contains(["Enabled", "Disabled"], var.status)
    error_message = "status must be Enabled or Disabled"
  }
}

variable "expiration_days" {
  type        = number
  default     = 30
  description = "Number of days after which objects matching the rule expire."

  validation {
    condition     = var.expiration_days >= 1
    error_message = "expiration_days must be at least 1"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  bucket_name = "tf-s3-lifecycle-${random_id.suffix.hex}"
  rule_id     = "expiration-rule"
  rule_prefix = "archive/"
}

resource "ctyun_oos_bucket" "test" {
  bucket = local.bucket_name
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = local.rule_id
    prefix = local.rule_prefix
    status = var.status

    expiration {
      days = var.expiration_days
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_lifecycle_configuration.test]
}

locals {
  resource_rule = one([
    for rule in ctyun_oos_bucket_lifecycle_configuration.test.rule : rule
    if rule.id == local.rule_id
  ])
  data_rule = one([
    for rule in data.ctyun_oos_bucket_lifecycle_configuration.test.rule : rule
    if rule.id == local.rule_id
  ])
}

output "bucket_name" {
  value = ctyun_oos_bucket.test.bucket
}

output "resource_rule_count" {
  value = length(ctyun_oos_bucket_lifecycle_configuration.test.rule)
}

output "resource_rule_id" {
  value = local.resource_rule.id
}

output "resource_rule_prefix" {
  value = local.resource_rule.prefix
}

output "resource_rule_status" {
  value = local.resource_rule.status
}

output "resource_rule_expiration_days" {
  value = local.resource_rule.expiration[0].days
}

output "data_lifecycle_configuration_id" {
  value = data.ctyun_oos_bucket_lifecycle_configuration.test.id
}

output "data_rule_count" {
  value = length(data.ctyun_oos_bucket_lifecycle_configuration.test.rule)
}

output "data_rule_id" {
  value = local.data_rule.id
}

output "data_rule_prefix" {
  value = local.data_rule.prefix
}

output "data_rule_status" {
  value = local.data_rule.status
}

output "data_rule_expiration_days" {
  value = local.data_rule.expiration[0].days
}
