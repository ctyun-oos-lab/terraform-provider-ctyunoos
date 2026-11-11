# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-LIFECYCLE-CLI-02 Invalid lifecycle inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider schema validation errors
# are not treated as expected failures, so these cases use the equivalent
# validation rules on the override variables.
run "rejects_invalid_status" {
  command = plan

  variables {
    status = "enabled"
  }

  expect_failures = [
    var.status,
  ]
}

run "rejects_non_positive_expiration_days" {
  command = plan

  variables {
    expiration_days = 0
  }

  expect_failures = [
    var.expiration_days,
  ]
}
