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

variable "attachment_user" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the attachment user name. Used by negative tests."

  validation {
    condition = (
      var.attachment_user == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.attachment_user))
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

variable "attachments_ds_user_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the attachments data source user_name. Used by negative tests."

  validation {
    condition = (
      var.attachments_ds_user_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.attachments_ds_user_name))
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
  user_name   = "${local.name_prefix}-user"
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

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_policy" "primary" {
  name        = "${local.name_prefix}-policy-a"
  description = "Primary policy for user policy attachment CLI test"

  policy = jsonencode(local.policy_document)
}

resource "ctyun_iam_policy" "secondary" {
  name        = "${local.name_prefix}-policy-b"
  description = "Secondary policy for user policy attachment CLI test"

  policy = jsonencode(local.policy_document)
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = coalesce(var.attachment_user, ctyun_iam_user.test.name)
  policy_arn = coalesce(var.attachment_policy_arn, local.selected_policy_arn)
}

data "ctyun_iam_user_policy_attachments" "test" {
  user_name = coalesce(var.attachments_ds_user_name, ctyun_iam_user.test.name)

  depends_on = [ctyun_iam_user_policy_attachment.test]
}

output "user_name" {
  value = ctyun_iam_user.test.name
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
  value = ctyun_iam_user_policy_attachment.test.id
}

output "attachment_user" {
  value = ctyun_iam_user_policy_attachment.test.user
}

output "attachment_policy_arn" {
  value = ctyun_iam_user_policy_attachment.test.policy_arn
}

output "expected_attachment_id" {
  value = "${ctyun_iam_user.test.name}/${local.selected_policy_arn}"
}

output "selected_policy_arn" {
  value = local.selected_policy_arn
}

output "selected_policy_name" {
  value = local.selected_policy_name
}

output "attachments_ds_id" {
  value = data.ctyun_iam_user_policy_attachments.test.id
}

output "attachments_ds_count" {
  value = length(data.ctyun_iam_user_policy_attachments.test.attached_policies)
}

output "attachments_ds_policy_arns" {
  value = toset(data.ctyun_iam_user_policy_attachments.test.attached_policies[*].policy_arn)
}

output "attachments_ds_policy_names" {
  value = toset(data.ctyun_iam_user_policy_attachments.test.attached_policies[*].policy_name)
}
