# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CROSS-COMPOSITE-04 IAM Role To S3 Bucket To CloudTrail Trail
run "creates_and_reads_role_bucket_cloudtrail_chain" {
  command = apply

  assert {
    condition     = output.policy_principal_ctyun == output.role_arn
    error_message = "The bucket policy principal does not match the created IAM role ARN."
  }

  assert {
    condition     = output.trail_bucket_name == output.bucket_name
    error_message = "The CloudTrail data source did not return the bucket configured for the trail."
  }

  assert {
    condition     = output.trail_logging_enabled == true
    error_message = "The CloudTrail data source did not report the default logging state as enabled."
  }
}

run "verifies_convergence" {
  command = plan
}
