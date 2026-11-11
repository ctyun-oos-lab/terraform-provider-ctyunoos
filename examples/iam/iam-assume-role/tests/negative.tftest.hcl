# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-ASSUME-ROLE-CLI-02 Invalid assume_role inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases mirror the provider rules
# on variables before the provider block evaluates them.
#
# Dummy access_key/secret_key satisfy required variables; validation fails
# before provider configure / AssumeRole.

run "rejects_duration_without_unit" {
  command = plan

  module {
    source = "./assumed"
  }

  variables {
    access_key = "AKIATESTPLACEHOLDER01"
    secret_key = "test-secret-placeholder"
    role_arn   = "arn:ctyun:iam::000000000000:role/placeholder"
    duration   = "3600"
  }

  expect_failures = [
    var.duration,
  ]
}

run "rejects_invalid_role_arn" {
  command = plan

  module {
    source = "./assumed"
  }

  variables {
    access_key = "AKIATESTPLACEHOLDER01"
    secret_key = "test-secret-placeholder"
    role_arn   = "not-an-arn"
  }

  expect_failures = [
    var.role_arn,
  ]
}

run "rejects_short_session_name" {
  command = plan

  module {
    source = "./assumed"
  }

  variables {
    access_key   = "AKIATESTPLACEHOLDER01"
    secret_key   = "test-secret-placeholder"
    role_arn     = "arn:ctyun:iam::000000000000:role/placeholder"
    session_name = "x"
  }

  expect_failures = [
    var.session_name,
  ]
}
