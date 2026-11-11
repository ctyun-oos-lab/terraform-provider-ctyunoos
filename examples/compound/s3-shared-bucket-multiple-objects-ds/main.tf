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
  description = "Existing shared OOS bucket used for multiple-object testing."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/multiple-objects/${random_id.suffix.hex}/"
  object_names  = ["test1.txt", "test2.txt"]
}

resource "ctyun_oos_object" "test" {
  count = 2

  bucket       = var.bucket_name
  key          = "${local.object_prefix}${local.object_names[count.index]}"
  content      = "terraform composite object ${count.index + 1}"
  content_type = "text/plain"
}

data "ctyun_oos_objects" "test" {
  bucket = var.bucket_name

  depends_on = [ctyun_oos_object.test]
}

output "bucket_name" {
  value = var.bucket_name
}

output "expected_object_keys" {
  value = toset([for object in ctyun_oos_object.test : object.key])
}

output "returned_object_keys" {
  value = toset(data.ctyun_oos_objects.test.keys)
}

output "object_buckets" {
  value = toset([for object in ctyun_oos_object.test : object.bucket])
}
