# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SOFT-DELETE-CLI-02 Retention days outside the OOS range are rejected at plan time.
run "rejects_retention_days_out_of_range" {
  command = plan
  variables {
    retention_days = 16
  }
  expect_failures = [
    var.retention_days,
  ]
}
