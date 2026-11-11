# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-GPA-CLI-02 Invalid group-policy attachment inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same rules on
# override variables.
run "rejects_invalid_attachment_group" {
  command = plan

  variables {
    attachment_group = "bad name"
  }

  expect_failures = [
    var.attachment_group,
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

run "rejects_invalid_attachments_ds_group_name" {
  command = plan

  variables {
    attachments_ds_group_name = "bad name"
  }

  expect_failures = [
    var.attachments_ds_group_name,
  ]
}
