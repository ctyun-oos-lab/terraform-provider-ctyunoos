# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}

provider "ctyun" {}

variable "allow_users_to_change_password" {
  type        = bool
  default     = false
  description = "Whether IAM users can change their own passwords."
}

variable "hard_expiry" {
  type        = bool
  default     = true
  description = "Whether users must obtain an administrator reset after passwords expire."
}

variable "max_password_age" {
  type        = number
  default     = 90
  description = "Maximum password age in days. Valid values are 0 through 1095."

  validation {
    condition     = var.max_password_age >= 0 && var.max_password_age <= 1095
    error_message = "max_password_age must be between 0 and 1095"
  }
}

variable "minimum_password_length" {
  type        = number
  default     = 16
  description = "Minimum password length. Valid values are 8 through 128."

  validation {
    condition     = var.minimum_password_length >= 8 && var.minimum_password_length <= 128
    error_message = "minimum_password_length must be between 8 and 128"
  }
}

variable "password_reuse_prevention" {
  type        = number
  default     = 5
  description = "Number of previous passwords that cannot be reused. Valid values are 0 through 24."

  validation {
    condition     = var.password_reuse_prevention >= 0 && var.password_reuse_prevention <= 24
    error_message = "password_reuse_prevention must be between 0 and 24"
  }
}

variable "require_lowercase_characters" {
  type        = bool
  default     = false
  description = "Whether passwords must include lowercase letters."
}

variable "require_numbers" {
  type        = bool
  default     = false
  description = "Whether passwords must include numbers."
}

variable "require_symbols" {
  type        = bool
  default     = true
  description = "Whether passwords must include symbols."
}

variable "require_uppercase_characters" {
  type        = bool
  default     = true
  description = "Whether passwords must include uppercase letters."
}

resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = var.allow_users_to_change_password
  hard_expiry                    = var.hard_expiry
  max_password_age               = var.max_password_age
  minimum_password_length        = var.minimum_password_length
  password_reuse_prevention      = var.password_reuse_prevention
  require_lowercase_characters   = var.require_lowercase_characters
  require_numbers                = var.require_numbers
  require_symbols                = var.require_symbols
  require_uppercase_characters   = var.require_uppercase_characters
}

data "ctyun_iam_account_password_policy" "test" {
  depends_on = [ctyun_iam_account_password_policy.test]
}

output "policy_id" {
  value = ctyun_iam_account_password_policy.test.id
}

output "allow_users_to_change_password" {
  value = ctyun_iam_account_password_policy.test.allow_users_to_change_password
}

output "expire_passwords" {
  value = ctyun_iam_account_password_policy.test.expire_passwords
}

output "hard_expiry" {
  value = ctyun_iam_account_password_policy.test.hard_expiry
}

output "max_password_age" {
  value = ctyun_iam_account_password_policy.test.max_password_age
}

output "minimum_password_length" {
  value = ctyun_iam_account_password_policy.test.minimum_password_length
}

output "password_reuse_prevention" {
  value = ctyun_iam_account_password_policy.test.password_reuse_prevention
}

output "require_lowercase_characters" {
  value = ctyun_iam_account_password_policy.test.require_lowercase_characters
}

output "require_numbers" {
  value = ctyun_iam_account_password_policy.test.require_numbers
}

output "require_symbols" {
  value = ctyun_iam_account_password_policy.test.require_symbols
}

output "require_uppercase_characters" {
  value = ctyun_iam_account_password_policy.test.require_uppercase_characters
}

output "data_policy_id" {
  value = data.ctyun_iam_account_password_policy.test.id
}

output "data_allow_users_to_change_password" {
  value = data.ctyun_iam_account_password_policy.test.allow_users_to_change_password
}

output "data_expire_passwords" {
  value = data.ctyun_iam_account_password_policy.test.expire_passwords
}

output "data_hard_expiry" {
  value = data.ctyun_iam_account_password_policy.test.hard_expiry
}

output "data_max_password_age" {
  value = data.ctyun_iam_account_password_policy.test.max_password_age
}

output "data_minimum_password_length" {
  value = data.ctyun_iam_account_password_policy.test.minimum_password_length
}

output "data_password_reuse_prevention" {
  value = data.ctyun_iam_account_password_policy.test.password_reuse_prevention
}

output "data_require_lowercase_characters" {
  value = data.ctyun_iam_account_password_policy.test.require_lowercase_characters
}

output "data_require_numbers" {
  value = data.ctyun_iam_account_password_policy.test.require_numbers
}

output "data_require_symbols" {
  value = data.ctyun_iam_account_password_policy.test.require_symbols
}

output "data_require_uppercase_characters" {
  value = data.ctyun_iam_account_password_policy.test.require_uppercase_characters
}
