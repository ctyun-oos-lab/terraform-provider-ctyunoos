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

variable "attachment_role" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the attachment role name. Used by negative tests."

  validation {
    condition = (
      var.attachment_role == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.attachment_role))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "attachment_policy_arn" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the attachment policy ARN. Used by negative tests."

  validation {
    condition = (
      var.attachment_policy_arn == null ? true : (
        length(var.attachment_policy_arn) >= 20 &&
        length(var.attachment_policy_arn) <= 2048
      )
    )
    error_message = "expected length of policy_arn to be in the range (20 - 2048)"
  }
}

variable "attachments_ds_role_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the attachments data source role_name. Used by negative tests."

  validation {
    condition = (
      var.attachments_ds_role_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.attachments_ds_role_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

variable "use_second_policy" {
  type        = bool
  default     = false
  description = "Attach the secondary policy instead of the primary policy. Triggers ForceNew replace."
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix = "tf-acc-test-${random_id.suffix.hex}"
  role_name   = "${local.name_prefix}-role"
  assume_role_policy = {
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = "*"
    }]
  }
  policy_document = {
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:ListUsers"]
      Resource = "*"
    }]
  }
  selected_policy_arn = (
    var.use_second_policy
    ? ctyun_iam_policy.secondary.arn
    : ctyun_iam_policy.primary.arn
  )
  selected_policy_name = (
    var.use_second_policy
    ? ctyun_iam_policy.secondary.name
    : ctyun_iam_policy.primary.name
  )
}

resource "ctyun_iam_role" "test" {
  name               = local.role_name
  assume_role_policy = jsonencode(local.assume_role_policy)
}

resource "ctyun_iam_policy" "primary" {
  name        = "${local.name_prefix}-policy-a"
  description = "Primary policy for role policy attachment CLI test"

  policy = jsonencode(local.policy_document)
}

resource "ctyun_iam_policy" "secondary" {
  name        = "${local.name_prefix}-policy-b"
  description = "Secondary policy for role policy attachment CLI test"

  policy = jsonencode(local.policy_document)
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = coalesce(var.attachment_role, ctyun_iam_role.test.name)
  policy_arn = coalesce(var.attachment_policy_arn, local.selected_policy_arn)
}

data "ctyun_iam_role_policy_attachments" "test" {
  role_name = coalesce(var.attachments_ds_role_name, ctyun_iam_role.test.name)

  depends_on = [ctyun_iam_role_policy_attachment.test]
}

output "role_name" {
  value = ctyun_iam_role.test.name
}

output "primary_policy_arn" {
  value = ctyun_iam_policy.primary.arn
}

output "primary_policy_name" {
  value = ctyun_iam_policy.primary.name
}

output "secondary_policy_arn" {
  value = ctyun_iam_policy.secondary.arn
}

output "secondary_policy_name" {
  value = ctyun_iam_policy.secondary.name
}

output "attachment_id" {
  value = ctyun_iam_role_policy_attachment.test.id
}

output "attachment_role" {
  value = ctyun_iam_role_policy_attachment.test.role
}

output "attachment_policy_arn" {
  value = ctyun_iam_role_policy_attachment.test.policy_arn
}

output "expected_attachment_id" {
  value = "${ctyun_iam_role.test.name}/${local.selected_policy_arn}"
}

output "selected_policy_arn" {
  value = local.selected_policy_arn
}

output "selected_policy_name" {
  value = local.selected_policy_name
}

output "attachments_ds_id" {
  value = data.ctyun_iam_role_policy_attachments.test.id
}

output "attachments_ds_count" {
  value = length(data.ctyun_iam_role_policy_attachments.test.attached_policies)
}

output "attachments_ds_policy_arns" {
  value = toset(data.ctyun_iam_role_policy_attachments.test.attached_policies[*].policy_arn)
}

output "attachments_ds_policy_names" {
  value = toset(data.ctyun_iam_role_policy_attachments.test.attached_policies[*].policy_name)
}
