# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COMPOSITE-02 Independent Subresources
run "creates_and_reads_independent_bucket_subresources" {
  command = apply

  assert {
    condition     = output.acl_bucket == output.bucket_name && output.acl_value == "private"
    error_message = "The bucket ACL did not read back private on the test bucket."
  }

  assert {
    condition     = output.policy_bucket == output.bucket_name && strcontains(output.policy_document, "oos:GetObject") && output.policy_resource == "arn:ctyun:oos:::${output.bucket_name}/*"
    error_message = "The bucket policy data source did not return the configured test-bucket policy."
  }

  assert {
    condition     = output.encryption_bucket == output.bucket_name && output.encryption_algorithm == "AES256"
    error_message = "The independent bucket encryption configuration did not read back AES256 on the test bucket."
  }
}

run "verifies_convergence" {
  command = plan
}
