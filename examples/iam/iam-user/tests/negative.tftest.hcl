# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-USER-CLI-02 Invalid user name is rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so this case uses the same character set
# rule on var.user_name.
run "rejects_invalid_user_name" {
  command = plan

  variables {
    user_name = "bad name"
  }

  expect_failures = [
    var.user_name,
  ]
}
