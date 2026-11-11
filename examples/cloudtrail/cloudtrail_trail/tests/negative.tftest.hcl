# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CT-TRAIL-CLI-02 CloudTrail input validation.

run "rejects_trail_name_too_short" {
  command = plan

  variables {
    trail_name = "ab"
  }

  expect_failures = [
    var.trail_name,
  ]
}

run "rejects_trail_name_too_long" {
  command = plan

  variables {
    trail_name = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }

  expect_failures = [
    var.trail_name,
  ]
}

run "rejects_prefix_over_200_characters" {
  command = plan

  variables {
    s3_key_prefix = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }

  expect_failures = [
    var.s3_key_prefix,
  ]
}

run "rejects_invalid_event_selector_mode" {
  command = plan

  variables {
    event_selector_enabled = true
    read_write_type        = "Invalid"
  }

  expect_failures = [
    var.read_write_type,
  ]
}
