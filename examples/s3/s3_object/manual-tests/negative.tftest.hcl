# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECT-CLI-02 Invalid object storage class is rejected at plan time.
run "rejects_invalid_storage_class" {
  command = plan

  variables {
    storage_class = "GLACIER"
  }

  expect_failures = [
    var.storage_class,
  ]
}
