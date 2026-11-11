# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SOFT-DELETE-CLI-01 Configure soft delete, read it through the data source, update it, then confirm no drift.
run "creates_and_reads_configuration" {
  command = apply

  assert {
    condition     = output.resource_id == output.bucket_name && output.data_id == output.bucket_name
    error_message = "The soft-delete resource and data source IDs should be the bucket name."
  }

  assert {
    condition     = output.resource_days == 7 && output.data_days == output.resource_days
    error_message = "The soft-delete retention period was not read back as 7 days by both paths."
  }
}

run "updates_retention_period" {
  command = apply
  variables {
    retention_days = 14
  }

  assert {
    condition     = output.resource_days == 14 && output.data_days == output.resource_days
    error_message = "Updating soft-delete retention did not read back 14 days."
  }
}

run "verifies_convergence" {
  command = plan
  variables {
    retention_days = 14
  }

  assert {
    condition     = output.resource_days == 14 && output.data_days == 14
    error_message = "Apply was followed by a plan that still expected a soft-delete configuration change."
  }
}
