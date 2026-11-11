# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-UPA-CLI-01 Create a user, policy, and attachment, switch the policy, then confirm no drift.
run "creates_and_reads_attachment" {
  command = apply

  assert {
    condition     = output.attachment_user == output.user_name
    error_message = "The attachment user does not match the created IAM user."
  }

  assert {
    condition     = output.attachment_policy_arn == output.primary_policy_arn
    error_message = "The attachment policy_arn does not match the primary policy."
  }

  assert {
    condition     = output.attachment_id == output.expected_attachment_id
    error_message = "The attachment ID should be <user-name>/<policy_arn>."
  }

  assert {
    condition     = output.attachments_ds_id == output.user_name
    error_message = "The attachments data source id should be the IAM user name."
  }

  assert {
    condition     = output.attachments_ds_count == 1
    error_message = "The attachments data source should return exactly one attached policy."
  }

  assert {
    condition     = contains(output.attachments_ds_policy_arns, output.primary_policy_arn)
    error_message = "The attachments data source did not return the attached primary policy ARN."
  }

  assert {
    condition     = contains(output.attachments_ds_policy_names, output.primary_policy_name)
    error_message = "The attachments data source did not return the attached primary policy name."
  }
}

run "switches_to_secondary_policy" {
  command = apply

  variables {
    use_second_policy = true
  }

  assert {
    condition     = output.attachment_policy_arn == output.secondary_policy_arn
    error_message = "Switching policy_arn did not attach the secondary policy."
  }

  assert {
    condition     = output.attachment_id == output.expected_attachment_id
    error_message = "The attachment ID should be <user-name>/<secondary-policy_arn> after the switch."
  }

  assert {
    condition     = output.attachments_ds_count == 1
    error_message = "The attachments data source should still return exactly one attached policy after the switch."
  }

  assert {
    condition     = contains(output.attachments_ds_policy_arns, output.secondary_policy_arn)
    error_message = "The attachments data source did not return the secondary policy ARN after the switch."
  }

  assert {
    condition     = contains(output.attachments_ds_policy_names, output.secondary_policy_name)
    error_message = "The attachments data source did not return the secondary policy name after the switch."
  }

  assert {
    condition     = !contains(output.attachments_ds_policy_arns, output.primary_policy_arn)
    error_message = "The primary policy was still attached after switching to the secondary policy."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    use_second_policy = true
  }

  assert {
    condition     = output.attachment_policy_arn == output.secondary_policy_arn
    error_message = "Apply was followed by a plan that still expected an attachment change."
  }
}
