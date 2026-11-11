# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECTS-CLI-02 Invalid object-listing arguments are rejected at plan time.
run "rejects_invalid_delimiter" {
  command = plan

  variables {
    delimiter = "-"
  }

  expect_failures = [
    var.delimiter,
  ]
}

run "rejects_non_positive_max_keys" {
  command = plan

  variables {
    max_keys = 0
  }

  expect_failures = [
    var.max_keys,
  ]
}
