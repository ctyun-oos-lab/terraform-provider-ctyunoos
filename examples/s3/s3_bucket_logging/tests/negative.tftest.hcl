# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-LOGGING-CLI-02 Invalid prefixes are rejected at plan time.
#
# Provider schema validation errors are not matched by terraform test
# expect_failures, so this test uses an equivalent validation on the variable.
run "rejects_invalid_target_prefix" {
  command = plan

  variables {
    target_prefix = "invalid-prefix"
  }

  expect_failures = [
    var.target_prefix,
  ]
}
