# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-LIFECYCLE-CLI-01 Create a lifecycle configuration, read it through the data source, update it, then confirm no drift.
run "creates_and_reads_lifecycle_configuration" {
  command = apply

  assert {
    condition     = output.data_lifecycle_configuration_id == output.bucket_name
    error_message = "The lifecycle configuration data source ID should be the bucket name."
  }

  assert {
    condition     = output.resource_rule_count == 1 && output.data_rule_count == 1
    error_message = "The resource or data source did not return exactly one lifecycle rule."
  }

  assert {
    condition     = output.resource_rule_id == "expiration-rule" && output.data_rule_id == output.resource_rule_id
    error_message = "The lifecycle rule ID was not preserved by the resource and data source."
  }

  assert {
    condition     = output.resource_rule_prefix == "archive/" && output.data_rule_prefix == output.resource_rule_prefix
    error_message = "The lifecycle rule prefix was not preserved by the resource and data source."
  }

  assert {
    condition     = output.resource_rule_status == "Enabled" && output.data_rule_status == output.resource_rule_status
    error_message = "The lifecycle rule status was not read back as Enabled."
  }

  assert {
    condition     = output.resource_rule_expiration_days == 30 && output.data_rule_expiration_days == output.resource_rule_expiration_days
    error_message = "The lifecycle expiration days were not read back as 30."
  }
}

run "updates_lifecycle_configuration" {
  command = apply

  variables {
    status          = "Disabled"
    expiration_days = 60
  }

  assert {
    condition     = output.resource_rule_status == "Disabled" && output.data_rule_status == output.resource_rule_status
    error_message = "Updating the lifecycle rule status did not read back Disabled."
  }

  assert {
    condition     = output.resource_rule_expiration_days == 60 && output.data_rule_expiration_days == output.resource_rule_expiration_days
    error_message = "Updating lifecycle expiration days did not read back 60."
  }

  assert {
    condition     = output.data_rule_id == output.resource_rule_id && output.data_rule_prefix == output.resource_rule_prefix
    error_message = "The lifecycle data source drifted after the update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    status          = "Disabled"
    expiration_days = 60
  }

  assert {
    condition     = output.resource_rule_status == "Disabled" && output.resource_rule_expiration_days == 60
    error_message = "Apply was followed by a plan that still expected a lifecycle configuration change."
  }
}
