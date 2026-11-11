# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COPY-CLI-02 Invalid copy storage class is rejected at plan time.
run "rejects_invalid_storage_class" {
  command = plan

  variables {
    target_storage_class = "GLACIER"
  }

  expect_failures = [
    var.target_storage_class,
  ]
}
