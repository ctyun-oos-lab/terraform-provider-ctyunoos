# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-GROUP-CLI-02 Invalid group name is rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so this case uses the same character set
# rule on var.group_name.
run "rejects_invalid_group_name" {
  command = plan

  variables {
    group_name = "bad name"
  }

  expect_failures = [
    var.group_name,
  ]
}
