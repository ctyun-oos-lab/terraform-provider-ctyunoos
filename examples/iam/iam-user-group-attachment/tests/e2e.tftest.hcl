# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-MEMBERSHIP-CLI-01 Create a user, group, and membership, then read the relation back.
run "creates_and_reads_membership" {
  command = apply

  assert {
    condition     = output.membership_user == output.user_name
    error_message = "The membership user does not match the created IAM user."
  }

  assert {
    condition     = output.membership_group == output.group_name
    error_message = "The membership group does not match the created IAM group."
  }

  assert {
    condition     = output.membership_id == output.expected_membership_id
    error_message = "The membership ID should be <user-name>/<group-name>."
  }

  assert {
    condition     = output.membership_ds_id == output.user_name
    error_message = "The membership data source id should be the IAM user name."
  }

  assert {
    condition     = output.membership_ds_group_count == 1
    error_message = "The membership data source should return exactly one group for the new user."
  }

  assert {
    condition     = contains(output.membership_ds_group_names, output.group_name)
    error_message = "The membership data source did not return the attached group name."
  }

  assert {
    condition     = contains(output.membership_ds_group_arns, output.group_arn)
    error_message = "The membership data source did not return the attached group ARN."
  }

  assert {
    condition     = contains(output.membership_ds_group_ids, output.group_unique_id)
    error_message = "The membership data source did not return the attached group unique_id."
  }

  assert {
    condition     = output.group_ds_user_count == 1
    error_message = "The group data source should list exactly one member after the attachment."
  }

  assert {
    condition     = contains(output.group_ds_user_names, output.user_name)
    error_message = "The group data source did not list the attached user."
  }
}

run "verifies_convergence" {
  command = plan

  assert {
    condition     = output.membership_id == output.expected_membership_id
    error_message = "Apply was followed by a plan that still expected a membership change."
  }
}
