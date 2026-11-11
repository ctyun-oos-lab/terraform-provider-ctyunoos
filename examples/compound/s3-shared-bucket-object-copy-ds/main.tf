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
  description = "Existing shared OOS bucket used for object-copy testing."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  object_prefix = "tf-acc-test/${random_id.suffix.hex}/"
  source_key    = "${local.object_prefix}source.txt"
  target_key    = "${local.object_prefix}copied.txt"
  object_body   = "terraform composite object copy source"
}

resource "ctyun_oos_object" "src" {
  bucket       = var.bucket_name
  key          = local.source_key
  content      = local.object_body
  content_type = "text/plain"
}

resource "ctyun_oos_object_copy" "test" {
  bucket = var.bucket_name
  key    = local.target_key
  source = "${ctyun_oos_object.src.bucket}/${ctyun_oos_object.src.key}"
}

data "ctyun_oos_object" "test" {
  bucket = var.bucket_name
  key    = ctyun_oos_object_copy.test.key

  depends_on = [ctyun_oos_object_copy.test]
}

output "bucket_name" {
  value = var.bucket_name
}

output "source_key" {
  value = ctyun_oos_object.src.key
}

output "source_bucket" {
  value = ctyun_oos_object.src.bucket
}

output "target_key" {
  value = ctyun_oos_object_copy.test.key
}

output "copy_bucket" {
  value = ctyun_oos_object_copy.test.bucket
}

output "expected_copy_source" {
  value = "${ctyun_oos_object.src.bucket}/${ctyun_oos_object.src.key}"
}

output "copy_source" {
  value = ctyun_oos_object_copy.test.source
}

output "source_etag" {
  value = ctyun_oos_object.src.etag
}

output "copy_etag" {
  value = ctyun_oos_object_copy.test.etag
}

output "object_ds_etag" {
  value = data.ctyun_oos_object.test.etag
}
