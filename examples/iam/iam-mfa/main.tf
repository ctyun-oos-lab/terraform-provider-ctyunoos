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

variable "virtual_mfa_device_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated virtual MFA device name. Used by negative tests."

  validation {
    condition = (
      var.virtual_mfa_device_name == null ||
      can(regex("^[0-9A-Za-z=,.@\\-_+]+$", var.virtual_mfa_device_name))
    )
    error_message = "must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name_prefix              = "tf-acc-test-${random_id.suffix.hex}"
  virtual_mfa_device_name  = coalesce(var.virtual_mfa_device_name, "${local.name_prefix}-mfa")
  user_name                = "${local.name_prefix}-mfa-user"
}

resource "ctyun_iam_user" "test" {
  name          = local.user_name
  force_destroy = true
}

resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = local.virtual_mfa_device_name
}

data "ctyun_iam_virtual_mfa_devices" "test" {
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}

data "ctyun_iam_mfa_devices" "by_user" {
  user_name = ctyun_iam_user.test.name
}

data "ctyun_iam_mfa_devices" "current" {}

output "device_name" {
  value = ctyun_iam_virtual_mfa_device.test.virtual_mfa_device_name
}

output "device_arn" {
  value = ctyun_iam_virtual_mfa_device.test.arn
}

output "device_id" {
  value = ctyun_iam_virtual_mfa_device.test.id
}

output "device_serial_number" {
  value = ctyun_iam_virtual_mfa_device.test.serial_number
}

output "device_user_name" {
  value = ctyun_iam_virtual_mfa_device.test.user_name
}

output "device_enable_date" {
  value = ctyun_iam_virtual_mfa_device.test.enable_date
}

output "virtual_mfa_ds_serials" {
  value = toset(data.ctyun_iam_virtual_mfa_devices.test.virtual_mfa_devices[*].serial_number)
}

output "virtual_mfa_ds_user_name" {
  value = one([
    for device in data.ctyun_iam_virtual_mfa_devices.test.virtual_mfa_devices : device.user.user_name
    if device.serial_number == ctyun_iam_virtual_mfa_device.test.serial_number
  ])
}

output "virtual_mfa_ds_enable_date" {
  value = one([
    for device in data.ctyun_iam_virtual_mfa_devices.test.virtual_mfa_devices : device.enable_date
    if device.serial_number == ctyun_iam_virtual_mfa_device.test.serial_number
  ])
}

output "mfa_devices_by_user_count" {
  value = length(data.ctyun_iam_mfa_devices.by_user.mfa_devices)
}

output "mfa_devices_current_count" {
  value = length(data.ctyun_iam_mfa_devices.current.mfa_devices)
}
