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

variable "allow_single_users_simultaneous_login" {
  type        = bool
  default     = false
  description = "Whether one IAM user can have simultaneous login sessions."
}

variable "lockout_duration" {
  type        = number
  default     = 20
  description = "Lockout duration in minutes. Valid values are 15 through 60."

  validation {
    condition     = var.lockout_duration >= 15 && var.lockout_duration <= 60
    error_message = "lockout_duration must be between 15 and 60"
  }
}

variable "login_failed_times" {
  type        = number
  default     = 7
  description = "Failed login attempts that trigger a lockout. Valid values are 5 through 10."

  validation {
    condition     = var.login_failed_times >= 5 && var.login_failed_times <= 10
    error_message = "login_failed_times must be between 5 and 10"
  }
}

variable "period_with_login_failures" {
  type        = number
  default     = 25
  description = "Failed-login measurement window in minutes. Valid values are 15 through 60."

  validation {
    condition     = var.period_with_login_failures >= 15 && var.period_with_login_failures <= 60
    error_message = "period_with_login_failures must be between 15 and 60"
  }
}

variable "session_duration" {
  type        = number
  default     = 25
  description = "Login session duration in minutes. Valid values are 10 through 30."

  validation {
    condition     = var.session_duration >= 10 && var.session_duration <= 30
    error_message = "session_duration must be between 10 and 30"
  }
}

resource "ctyun_iam_account_login_security_policy" "test" {
  allow_single_users_simultaneous_login = var.allow_single_users_simultaneous_login
  lockout_duration                      = var.lockout_duration
  login_failed_times                    = var.login_failed_times
  period_with_login_failures            = var.period_with_login_failures
  session_duration                      = var.session_duration
}

data "ctyun_iam_account_login_security_policy" "test" {
  depends_on = [ctyun_iam_account_login_security_policy.test]
}

output "policy_id" {
  value = ctyun_iam_account_login_security_policy.test.id
}

output "allow_single_users_simultaneous_login" {
  value = ctyun_iam_account_login_security_policy.test.allow_single_users_simultaneous_login
}

output "lockout_duration" {
  value = ctyun_iam_account_login_security_policy.test.lockout_duration
}

output "login_failed_times" {
  value = ctyun_iam_account_login_security_policy.test.login_failed_times
}

output "period_with_login_failures" {
  value = ctyun_iam_account_login_security_policy.test.period_with_login_failures
}

output "session_duration" {
  value = ctyun_iam_account_login_security_policy.test.session_duration
}

output "data_policy_id" {
  value = data.ctyun_iam_account_login_security_policy.test.id
}

output "data_allow_single_users_simultaneous_login" {
  value = data.ctyun_iam_account_login_security_policy.test.allow_single_users_simultaneous_login
}

output "data_lockout_duration" {
  value = data.ctyun_iam_account_login_security_policy.test.lockout_duration
}

output "data_login_failed_times" {
  value = data.ctyun_iam_account_login_security_policy.test.login_failed_times
}

output "data_period_with_login_failures" {
  value = data.ctyun_iam_account_login_security_policy.test.period_with_login_failures
}

output "data_session_duration" {
  value = data.ctyun_iam_account_login_security_policy.test.session_duration
}
