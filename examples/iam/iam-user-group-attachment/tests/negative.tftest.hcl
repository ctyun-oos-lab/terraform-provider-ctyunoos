# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-MEMBERSHIP-CLI-02 Invalid membership names are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same character set
# rule on the override variables.
run "rejects_invalid_membership_user" {
  command = plan

  variables {
    membership_user = "bad name"
  }

  expect_failures = [
    var.membership_user,
  ]
}

run "rejects_invalid_membership_group" {
  command = plan

  variables {
    membership_group = "bad name"
  }

  expect_failures = [
    var.membership_group,
  ]
}

run "rejects_invalid_membership_ds_user_name" {
  command = plan

  variables {
    membership_ds_user_name = "bad name"
  }

  expect_failures = [
    var.membership_ds_user_name,
  ]
}
