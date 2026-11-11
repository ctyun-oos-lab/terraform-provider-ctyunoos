# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-APP-CLI-02 Invalid numeric bounds are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same range rules
# on the override variables.
run "rejects_minimum_password_length_too_low" {
  command = plan

  variables {
    minimum_password_length = 7
  }

  expect_failures = [
    var.minimum_password_length,
  ]
}

run "rejects_max_password_age_too_high" {
  command = plan

  variables {
    max_password_age = 1096
  }

  expect_failures = [
    var.max_password_age,
  ]
}

run "rejects_password_reuse_prevention_too_high" {
  command = plan

  variables {
    password_reuse_prevention = 25
  }

  expect_failures = [
    var.password_reuse_prevention,
  ]
}
