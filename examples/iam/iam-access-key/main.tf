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
  default     = "Active"
  description = "Access key status. Valid values are Active and Inactive."

  validation {
    condition     = contains(["Active", "Inactive"], var.status)
    error_message = "status must be Active or Inactive"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  user_name = "tf-acc-test-${random_id.suffix.hex}-ak-user"
}

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_access_key" "test" {
  user   = ctyun_iam_user.test.name
  status = var.status
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_access_key.test]
}

output "user_name" {
  value = ctyun_iam_user.test.name
}

output "access_key_id" {
  value = ctyun_iam_access_key.test.id
}

output "access_key_user" {
  value = ctyun_iam_access_key.test.user
}

output "access_key_status" {
  value = ctyun_iam_access_key.test.status
}

output "access_key_create_date" {
  value = ctyun_iam_access_key.test.create_date
}

output "access_keys_ds_count" {
  value = length(data.ctyun_iam_access_keys.test.access_keys)
}

output "access_keys_ds_ids" {
  value = toset([
    for access_key in data.ctyun_iam_access_keys.test.access_keys : access_key.access_key_id
  ])
}

output "access_keys_ds_status" {
  value = one([
    for access_key in data.ctyun_iam_access_keys.test.access_keys : access_key.status
    if access_key.access_key_id == ctyun_iam_access_key.test.id
  ])
}
