# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CROSS-COMPOSITE-03 S3 Bucket To CloudTrail Trail
run "creates_and_reads_bucket_cloudtrail_chain" {
  command = apply

  assert {
    condition     = output.trail_bucket_name == output.bucket_name
    error_message = "The CloudTrail data source did not return the S3 bucket configured for the trail."
  }

  assert {
    condition     = output.trail_logging_enabled == true
    error_message = "The CloudTrail data source did not report the default logging state as enabled."
  }
}

run "verifies_convergence" {
  command = plan
}
