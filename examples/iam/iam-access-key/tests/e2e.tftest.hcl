# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-AK-CLI-01 Create an access key, read it through the data source, update status, then confirm no drift.
run "creates_and_reads_access_key" {
  command = apply

  assert {
    condition     = output.access_key_user == output.user_name
    error_message = "The access key user does not match the created IAM user."
  }

  assert {
    condition     = output.access_key_id != ""
    error_message = "The access key ID was empty after apply."
  }

  assert {
    condition     = output.access_key_create_date != ""
    error_message = "The access key create_date was empty after apply."
  }

  assert {
    condition     = output.access_key_status == "Active"
    error_message = "The access key resource did not read back Active status."
  }

  assert {
    condition     = output.access_keys_ds_count == 1
    error_message = "The access keys data source did not return exactly one key for the test user."
  }

  assert {
    condition     = contains(output.access_keys_ds_ids, output.access_key_id)
    error_message = "The access keys data source did not return the created access key ID."
  }

  assert {
    condition     = output.access_keys_ds_status == "Active"
    error_message = "The access keys data source did not return Active status for the created key."
  }
}

run "updates_status_inactive" {
  command = apply

  variables {
    status = "Inactive"
  }

  assert {
    condition     = output.access_key_status == "Inactive"
    error_message = "Updating status did not read back Inactive on the access key resource."
  }

  assert {
    condition     = output.access_keys_ds_status == "Inactive"
    error_message = "The access keys data source did not return Inactive after the status update."
  }

  assert {
    condition     = contains(output.access_keys_ds_ids, output.access_key_id)
    error_message = "The access keys data source lost the created access key ID after the status update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    status = "Inactive"
  }

  assert {
    condition     = output.access_key_status == "Inactive"
    error_message = "Apply was followed by a plan that still expected an access key change."
  }
}
