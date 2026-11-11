# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-ACL-CLI-02 Invalid canned ACL is rejected at plan time.
run "rejects_invalid_acl" {
  command = plan

  variables {
    acl = "authenticated-read"
  }

  expect_failures = [
    var.acl,
  ]
}
