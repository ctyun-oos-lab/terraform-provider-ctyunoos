# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-MFA-CLI-02 Invalid virtual MFA device name is rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so this case uses the same character set
# rule on var.virtual_mfa_device_name.
run "rejects_invalid_device_name" {
  command = plan

  variables {
    virtual_mfa_device_name = "bad name"
  }

  expect_failures = [
    var.virtual_mfa_device_name,
  ]
}
