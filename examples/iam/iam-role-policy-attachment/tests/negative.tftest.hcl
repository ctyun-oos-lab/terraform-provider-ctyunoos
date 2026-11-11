# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-RPA-CLI-02 Invalid role-policy attachment inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same rules on
# override variables.
run "rejects_invalid_attachment_role" {
  command = plan

  variables {
    attachment_role = "bad name"
  }

  expect_failures = [
    var.attachment_role,
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

run "rejects_invalid_attachments_ds_role_name" {
  command = plan

  variables {
    attachments_ds_role_name = "bad name"
  }

  expect_failures = [
    var.attachments_ds_role_name,
  ]
}
