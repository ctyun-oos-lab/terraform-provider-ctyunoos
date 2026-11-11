# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-LOGGING-CLI-01 Configure bucket logging, update its prefix, then confirm no drift.
run "creates_and_reads_logging" {
  command = apply

  assert {
    condition     = output.logging_id == output.logging_bucket && output.logging_bucket == output.source_bucket
    error_message = "The bucket logging resource ID or source bucket was not read back correctly."
  }

  assert {
    condition     = output.logging_target_bucket == output.target_bucket
    error_message = "The logging target bucket was not read back correctly."
  }

  assert {
    condition     = output.logging_target_prefix == "access-logs/"
    error_message = "The logging target prefix was not read back correctly."
  }
}

run "updates_logging_prefix" {
  command = apply

  variables {
    target_prefix = "terraform-logs/"
  }

  assert {
    condition     = output.logging_id == output.logging_bucket && output.logging_target_bucket == output.target_bucket
    error_message = "Updating the logging prefix changed the resource identity or target bucket."
  }

  assert {
    condition     = output.logging_target_prefix == "terraform-logs/"
    error_message = "Updating target_prefix did not read back the new value."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    target_prefix = "terraform-logs/"
  }

  assert {
    condition     = output.logging_target_prefix == "terraform-logs/"
    error_message = "Apply was followed by a plan that still expected a bucket logging change."
  }
}
