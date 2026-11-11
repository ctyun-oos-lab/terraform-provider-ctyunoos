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

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  user_name = "tf-acc-test-${random_id.suffix.hex}-user"
}

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_access_key" "test" {
  user = ctyun_iam_user.test.name
}

data "ctyun_iam_users" "test" {
  name_regex = "^${local.user_name}$"

  depends_on = [ctyun_iam_user.test]
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_access_key.test]
}

output "access_key_status" {
  value = ctyun_iam_access_key.test.status
}

output "expected_user_names" {
  value = toset([local.user_name])
}

output "returned_access_key_status" {
  value = one([
    for access_key in data.ctyun_iam_access_keys.test.access_keys : access_key.status
    if access_key.access_key_id == ctyun_iam_access_key.test.id
  ])
}

output "returned_user_names" {
  value = data.ctyun_iam_users.test.names
}
