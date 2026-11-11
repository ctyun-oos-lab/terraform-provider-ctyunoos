# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-GROUP-CLI-01 Create a group, read it through both data sources, then confirm no drift.
run "creates_and_reads_group" {
  command = apply

  assert {
    condition     = output.group_name == output.data_group_name
    error_message = "The IAM group data source name does not match the created group."
  }

  assert {
    condition     = output.group_arn == output.data_group_arn
    error_message = "The IAM group data source ARN does not match the created group."
  }

  assert {
    condition     = output.group_id == output.group_name
    error_message = "The IAM group resource ID should be the group name."
  }

  assert {
    condition     = output.group_unique_id != ""
    error_message = "The IAM group unique_id was empty after apply."
  }

  assert {
    condition     = output.data_group_id == output.group_unique_id
    error_message = "The IAM group data source group_id does not match the resource unique_id."
  }

  assert {
    condition     = output.data_group_user_count == 0
    error_message = "A newly created group should have no members."
  }

  assert {
    condition     = output.groups_ds_names == output.expected_group_names
    error_message = "The IAM groups data source did not return exactly the created group name."
  }

  assert {
    condition     = output.groups_ds_arns == output.expected_group_arns
    error_message = "The IAM groups data source did not return exactly the created group ARN."
  }

  assert {
    condition     = contains(output.groups_ds_group_ids, output.group_unique_id)
    error_message = "The IAM groups data source did not return the created group unique_id."
  }
}

run "verifies_convergence" {
  command = plan

  assert {
    condition     = output.group_name == output.data_group_name
    error_message = "Apply was followed by a plan that still expected a group change."
  }
}
