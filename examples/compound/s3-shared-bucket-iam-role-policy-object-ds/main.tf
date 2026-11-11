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
  description = "Existing shared OOS bucket used for role-policy object testing."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"
  role_name   = "${local.name_prefix}-role"
  object_key  = "tf-acc-test/iam-role-policy/${random_id.suffix.hex}/object.txt"
  object_body = "terraform IAM role policy object"
}

resource "ctyun_iam_role" "test" {
  name = local.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
      Effect = "Allow"
    }]
  })
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = var.bucket_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        CTYUN = ctyun_iam_role.test.arn
      }
      Action   = ["oos:GetObject"]
      Resource = "arn:ctyun:oos:::${var.bucket_name}/*"
    }]
  })

  depends_on = [ctyun_iam_role.test]
}

resource "ctyun_oos_object" "test" {
  bucket       = var.bucket_name
  key          = local.object_key
  content      = local.object_body
  content_type = "text/plain"

  depends_on = [ctyun_oos_bucket_policy.test]
}

data "ctyun_oos_bucket_policy" "test" {
  bucket = var.bucket_name

  depends_on = [ctyun_oos_bucket_policy.test]
}

data "ctyun_oos_object" "test" {
  bucket = var.bucket_name
  key    = ctyun_oos_object.test.key

  depends_on = [ctyun_oos_object.test]
}

output "bucket_name" {
  value = var.bucket_name
}

output "role_arn" {
  value = ctyun_iam_role.test.arn
}

output "policy_principal_ctyun" {
  value = jsondecode(data.ctyun_oos_bucket_policy.test.policy).Statement[0].Principal.CTYUN
}

output "object_key" {
  value = ctyun_oos_object.test.key
}

output "object_etag" {
  value = ctyun_oos_object.test.etag
}

output "expected_object_length" {
  value = length(local.object_body)
}

output "object_ds_etag" {
  value = data.ctyun_oos_object.test.etag
}

output "object_ds_content_length" {
  value = data.ctyun_oos_object.test.content_length
}
