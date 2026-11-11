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
  description = "Existing shared OOS bucket used for object lifecycle testing."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/${random_id.suffix.hex}/"
  object_key    = "${local.object_prefix}hello.txt"
  object_body   = "terraform composite object"
}

resource "ctyun_oos_object" "test" {
  bucket       = var.bucket_name
  key          = local.object_key
  content      = local.object_body
  content_type = "text/plain"
}

data "ctyun_oos_objects" "test" {
  bucket = var.bucket_name
  prefix = local.object_prefix

  depends_on = [ctyun_oos_object.test]
}

data "ctyun_oos_object" "test" {
  bucket = var.bucket_name
  key    = ctyun_oos_object.test.key

  depends_on = [ctyun_oos_object.test]
}

output "expected_object_key" {
  value = local.object_key
}

output "expected_object_length" {
  value = length(local.object_body)
}

output "object_etag" {
  value = ctyun_oos_object.test.etag
}

output "object_key" {
  value = ctyun_oos_object.test.key
}

output "objects_ds_keys" {
  value = toset(data.ctyun_oos_objects.test.keys)
}

output "object_ds_content_length" {
  value = data.ctyun_oos_object.test.content_length
}

output "object_ds_etag" {
  value = data.ctyun_oos_object.test.etag
}
