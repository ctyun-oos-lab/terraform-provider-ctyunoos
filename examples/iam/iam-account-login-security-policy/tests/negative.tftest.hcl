# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-ALSP-CLI-02 Invalid numeric bounds are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same range rules
# on the override variables.
run "rejects_lockout_duration_too_low" {
  command = plan

  variables {
    lockout_duration = 14
  }

  expect_failures = [
    var.lockout_duration,
  ]
}

run "rejects_login_failed_times_too_high" {
  command = plan

  variables {
    login_failed_times = 11
  }

  expect_failures = [
    var.login_failed_times,
  ]
}

run "rejects_period_with_login_failures_too_low" {
  command = plan

  variables {
    period_with_login_failures = 14
  }

  expect_failures = [
    var.period_with_login_failures,
  ]
}

run "rejects_session_duration_too_high" {
  command = plan

  variables {
    session_duration = 31
  }

  expect_failures = [
    var.session_duration,
  ]
}
