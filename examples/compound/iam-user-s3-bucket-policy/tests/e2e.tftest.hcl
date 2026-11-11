# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CROSS-COMPOSITE-01 IAM User To S3 Bucket Policy
run "creates_and_reads_iam_user_bucket_policy" {
  command = apply

  assert {
    condition     = output.policy_bucket == output.bucket_name
    error_message = "The bucket policy data source did not return the policy for the test bucket."
  }

  assert {
    condition     = output.policy_principal_ctyun == output.user_arn
    error_message = "The bucket policy principal does not match the created IAM user ARN."
  }

  assert {
    condition     = output.policy_resource == "arn:ctyun:oos:::${output.bucket_name}/*"
    error_message = "The bucket policy resource does not match the test bucket object ARN."
  }
}

run "verifies_convergence" {
  command = plan
}
