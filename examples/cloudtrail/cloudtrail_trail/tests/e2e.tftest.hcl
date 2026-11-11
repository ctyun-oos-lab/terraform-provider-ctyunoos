# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CT-TRAIL-CLI-01 CloudTrail resource and data source lifecycle.

run "creates_trail_and_reads_defaults" {
  command = apply

  assert {
    condition     = output.trail_id == output.data_trail_id && output.trail_id != ""
    error_message = "The CloudTrail resource and data source IDs did not match."
  }

  assert {
    condition     = output.trail_arn == output.data_trail_arn && output.trail_name == output.data_trail_name
    error_message = "The CloudTrail ARN or name was not read back consistently."
  }

  assert {
    condition     = output.trail_bucket_name == output.bucket_name && output.data_trail_bucket_name == output.bucket_name
    error_message = "The CloudTrail target bucket was not read back correctly."
  }

  assert {
    condition     = output.trail_enable_logging == true && output.data_trail_enable_logging == true
    error_message = "CloudTrail logging should be enabled by the initial configuration."
  }

  assert {
    condition     = output.trail_event_selector_count >= 1 && output.data_trail_event_selector_count >= 1 && output.trail_read_write_type == "All" && output.data_trail_read_write_type == "All"
    error_message = "The CloudTrail default event selector was not read back as All."
  }
}

run "updates_logging_selector_and_prefix" {
  command = apply

  variables {
    enable_logging         = false
    event_selector_enabled = true
    read_write_type        = "ReadOnly"
    s3_key_prefix          = "audit/logs/"
  }

  assert {
    condition     = output.trail_enable_logging == false && output.data_trail_enable_logging == false
    error_message = "Disabling CloudTrail logging was not read back correctly."
  }

  assert {
    condition     = output.trail_key_prefix == "audit/logs/" && output.data_trail_key_prefix == "audit/logs/"
    error_message = "The CloudTrail log prefix was not read back correctly."
  }

  assert {
    condition     = output.trail_event_selector_count == 1 && output.trail_read_write_type == "ReadOnly" && output.data_trail_read_write_type == "ReadOnly"
    error_message = "The ReadOnly event selector was not read back correctly."
  }
}

run "restarts_logging_and_changes_selector" {
  command = apply

  variables {
    enable_logging         = true
    event_selector_enabled = true
    read_write_type        = "WriteOnly"
    s3_key_prefix          = "audit/logs/"
  }

  assert {
    condition     = output.trail_enable_logging == true && output.data_trail_enable_logging == true
    error_message = "Re-enabling CloudTrail logging was not read back correctly."
  }

  assert {
    condition     = output.trail_read_write_type == "WriteOnly" && output.data_trail_read_write_type == "WriteOnly"
    error_message = "The WriteOnly event selector was not read back correctly."
  }
}

run "omits_selector_and_uses_cloud_value" {
  command = apply

  variables {
    enable_logging         = true
    event_selector_enabled = false
    read_write_type        = "All"
    s3_key_prefix          = "audit/logs/"
  }

  assert {
    condition     = output.trail_event_selector_count >= 1 && output.data_trail_event_selector_count >= 1 && output.trail_read_write_type == output.data_trail_read_write_type
    error_message = "Removing the optional selector block should preserve and read back the cloud selector."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    enable_logging         = true
    event_selector_enabled = false
    read_write_type        = "All"
    s3_key_prefix          = "audit/logs/"
  }

  assert {
    condition     = output.trail_id == output.data_trail_id && output.trail_enable_logging == true
    error_message = "The follow-up plan did not retain the stable CloudTrail state."
  }
}
