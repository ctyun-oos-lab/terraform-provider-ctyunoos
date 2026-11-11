# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SOFT-OBJECTS-CLI-02 Invalid soft-delete listing arguments are rejected at plan time.
run "rejects_non_positive_max_keys" {
  command = plan

  variables {
    max_keys = 0
  }

  expect_failures = [
    var.max_keys,
  ]
}
