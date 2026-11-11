# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECT-LOCK-CLI-01 Create an object-lock configuration, read it through the data source, update retention, then confirm no drift.
run "creates_and_reads_object_lock_configuration" {
  command = apply

  assert {
    condition     = output.resource_id == output.bucket_name && output.data_id == output.bucket_name
    error_message = "The object-lock resource and data source IDs should be the bucket name."
  }

  assert {
    condition     = output.resource_object_lock_enabled == "Disabled" && output.data_object_lock_enabled == output.resource_object_lock_enabled
    error_message = "Object Lock status was not read back as Disabled by the resource and data source."
  }

  assert {
    condition     = output.resource_rule_count == 1 && output.data_rule_count == 1
    error_message = "The resource or data source did not return exactly one retention rule."
  }

  assert {
    condition     = output.resource_retention_mode == "COMPLIANCE" && output.data_retention_mode == output.resource_retention_mode
    error_message = "The default retention mode was not read back as COMPLIANCE."
  }

  assert {
    condition     = output.resource_retention_days == 30 && output.data_retention_days == output.resource_retention_days
    error_message = "The default retention period was not read back as 30 days."
  }
}

run "updates_retention_period" {
  command = apply

  variables {
    retention_days = 60
  }

  assert {
    condition     = output.resource_retention_days == 60 && output.data_retention_days == output.resource_retention_days
    error_message = "Updating the retention period did not read back 60 days."
  }

  assert {
    condition     = output.resource_object_lock_enabled == "Disabled" && output.data_object_lock_enabled == "Disabled"
    error_message = "Object Lock status changed unexpectedly while updating retention."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    retention_days = 60
  }

  assert {
    condition     = output.resource_retention_days == 60 && output.data_retention_days == 60
    error_message = "Apply was followed by a plan that still expected an object-lock configuration change."
  }
}
