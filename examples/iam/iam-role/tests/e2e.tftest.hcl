# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-ROLE-CLI-01 Create a role, read it through both data sources, update, then confirm no drift.
run "creates_and_reads_role" {
  command = apply

  assert {
    condition     = output.role_name == output.data_role_name
    error_message = "The IAM role data source name does not match the created role."
  }

  assert {
    condition     = output.role_arn == output.data_role_arn
    error_message = "The IAM role data source ARN does not match the created role."
  }

  assert {
    condition     = output.role_id == output.role_name
    error_message = "The IAM role resource ID should be the role name."
  }

  assert {
    condition     = output.role_unique_id != ""
    error_message = "The IAM role unique_id was empty after apply."
  }

  assert {
    condition     = output.data_role_unique_id == output.role_unique_id
    error_message = "The IAM role data source unique_id does not match the resource unique_id."
  }

  assert {
    condition     = output.role_description == "Role managed by Terraform"
    error_message = "The IAM role description was not written back."
  }

  assert {
    condition     = output.data_role_description == output.role_description
    error_message = "The IAM role data source description does not match the resource."
  }

  assert {
    condition     = output.role_max_session_duration == 3600
    error_message = "The IAM role max_session_duration was not written back as 3600."
  }

  assert {
    condition     = output.data_role_max_session_duration == output.role_max_session_duration
    error_message = "The IAM role data source max_session_duration does not match the resource."
  }

  assert {
    condition     = output.role_assume_effect == "Allow" && output.data_role_assume_effect == "Allow"
    error_message = "The assume_role_policy Effect was not Allow on the resource or data source."
  }

  assert {
    condition     = output.roles_ds_names == output.expected_role_names
    error_message = "The IAM roles data source did not return exactly the created role name."
  }

  assert {
    condition     = output.roles_ds_arns == output.expected_role_arns
    error_message = "The IAM roles data source did not return exactly the created role ARN."
  }

  assert {
    condition     = contains(output.roles_ds_unique_ids, output.role_unique_id)
    error_message = "The IAM roles data source did not return the created role unique_id."
  }
}

run "updates_description_and_duration" {
  command = apply

  variables {
    description          = "Role updated by Terraform"
    max_session_duration = 7200
  }

  assert {
    condition     = output.role_description == "Role updated by Terraform"
    error_message = "Updating the role description did not read back the new value."
  }

  assert {
    condition     = output.data_role_description == "Role updated by Terraform"
    error_message = "The IAM role data source description drifted after the update."
  }

  assert {
    condition     = output.role_max_session_duration == 7200
    error_message = "Updating max_session_duration did not read back 7200."
  }

  assert {
    condition     = output.data_role_max_session_duration == 7200
    error_message = "The IAM role data source max_session_duration drifted after the update."
  }

  assert {
    condition     = output.roles_ds_names == output.expected_role_names
    error_message = "The IAM roles data source name list drifted after the update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    description          = "Role updated by Terraform"
    max_session_duration = 7200
  }

  assert {
    condition     = output.role_description == "Role updated by Terraform"
    error_message = "Apply was followed by a plan that still expected a role change."
  }
}
