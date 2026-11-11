# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SSE-CLI-02 OOS rejects encryption algorithms other than AES256.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider schema validation errors
# are not treated as expected failures, so this case uses the same restriction
# on the override variable.
run "rejects_unsupported_algorithm" {
  command = plan

  variables {
    sse_algorithm = "aws:kms"
  }

  expect_failures = [
    var.sse_algorithm,
  ]
}
