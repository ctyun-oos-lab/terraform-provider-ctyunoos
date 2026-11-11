# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-RECOVER-CLI-02 Invalid encoding type is rejected at plan time.
run "rejects_invalid_encoding_type" {
  command = plan

  variables {
    encoding_type = "xml"
  }

  expect_failures = [
    var.encoding_type,
  ]
}
