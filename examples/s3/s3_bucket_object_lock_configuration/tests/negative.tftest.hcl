# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECT-LOCK-CLI-02 Invalid object-lock inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider schema validation errors
# are not treated as expected failures, so these cases use equivalent
# validation rules on the override variables.
run "rejects_invalid_object_lock_status" {
  command = plan

  variables {
    object_lock_enabled = "enabled"
  }

  expect_failures = [
    var.object_lock_enabled,
  ]
}

run "rejects_retention_days_out_of_range" {
  command = plan

  variables {
    retention_days = 0
  }

  expect_failures = [
    var.retention_days,
  ]
}
