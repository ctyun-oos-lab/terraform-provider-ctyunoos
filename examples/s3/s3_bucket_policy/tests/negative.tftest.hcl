# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-POLICY-CLI-02 Invalid policy inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider schema validation errors
# are not treated as expected failures, so these cases use equivalent
# validation rules on the override variables.
run "rejects_invalid_action" {
  command = plan

  variables {
    action = "s3:GetObject"
  }

  expect_failures = [
    var.action,
  ]
}

run "rejects_invalid_effect" {
  command = plan

  variables {
    effect = "Permit"
  }

  expect_failures = [
    var.effect,
  ]
}
