# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-AK-CLI-02 Invalid status is rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so this case uses the same enum rule on
# var.status.
run "rejects_invalid_status" {
  command = plan

  variables {
    status = "active"
  }

  expect_failures = [
    var.status,
  ]
}
