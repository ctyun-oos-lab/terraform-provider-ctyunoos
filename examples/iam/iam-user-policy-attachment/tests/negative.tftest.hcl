# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-UPA-CLI-02 Invalid user-policy attachment inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same rules on
# override variables.
run "rejects_invalid_attachment_user" {
  command = plan

  variables {
    attachment_user = "bad name"
  }

  expect_failures = [
    var.attachment_user,
  ]
}

run "rejects_short_policy_arn" {
  command = plan

  variables {
    attachment_policy_arn = "too-short"
  }

  expect_failures = [
    var.attachment_policy_arn,
  ]
}

run "rejects_invalid_attachments_ds_user_name" {
  command = plan

  variables {
    attachments_ds_user_name = "bad name"
  }

  expect_failures = [
    var.attachments_ds_user_name,
  ]
}
