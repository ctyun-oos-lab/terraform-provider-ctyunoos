# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CROSS-COMPOSITE-02 IAM Role To S3 Bucket Policy With Object
run "creates_and_reads_iam_role_bucket_policy" {
  command = apply

  assert {
    condition     = output.policy_bucket == output.bucket_name
    error_message = "The bucket policy data source did not return the policy for the test bucket."
  }

  assert {
    condition     = output.policy_principal_ctyun == output.role_arn
    error_message = "The bucket policy principal does not match the created IAM role ARN."
  }
}

run "verifies_convergence" {
  command = plan
}
