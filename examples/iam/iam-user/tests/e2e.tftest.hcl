# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-USER-CLI-01 Create, read, update tags, then confirm no drift.
run "creates_and_reads_user" {
  command = apply

  assert {
    condition     = output.user_name == output.data_user_name
    error_message = "The IAM user data source name does not match the created user."
  }

  assert {
    condition     = output.user_arn == output.data_user_arn
    error_message = "The IAM user data source ARN does not match the created user."
  }

  assert {
    condition     = output.user_id == output.user_name
    error_message = "The IAM user resource ID should be the user name."
  }

  assert {
    condition     = output.user_unique_id != ""
    error_message = "The IAM user unique_id was empty after apply."
  }

  assert {
    condition     = output.managed_by == "terraform"
    error_message = "The IAM user managed_by tag was not written back as terraform."
  }

  assert {
    condition     = output.users_ds_names == output.expected_user_names
    error_message = "The IAM users data source did not return exactly the created user name."
  }

  assert {
    condition     = output.users_ds_arns == output.expected_user_arns
    error_message = "The IAM users data source did not return exactly the created user ARN."
  }

  assert {
    condition     = contains(output.users_ds_user_ids, output.user_unique_id)
    error_message = "The IAM users data source did not return the created user unique_id."
  }
}

run "updates_tags" {
  command = apply

  variables {
    managed_by = "terraform-updated"
  }

  assert {
    condition     = output.managed_by == "terraform-updated"
    error_message = "Updating the managed_by tag did not read back the new value."
  }

  assert {
    condition     = output.user_arn == output.data_user_arn
    error_message = "The IAM user data source ARN drifted after the tag update."
  }

  assert {
    condition     = output.users_ds_names == output.expected_user_names
    error_message = "The IAM users data source name list drifted after the tag update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    managed_by = "terraform-updated"
  }

  assert {
    condition     = output.managed_by == "terraform-updated"
    error_message = "Apply was followed by a plan that still expected a tag change."
  }
}
