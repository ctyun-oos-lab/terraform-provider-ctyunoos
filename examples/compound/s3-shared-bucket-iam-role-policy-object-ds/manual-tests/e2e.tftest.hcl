# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-CROSS-COMPOSITE-02 Object And Object Data Source
run "creates_and_reads_role_policy_object" {
  command = apply

  assert {
    condition     = output.policy_principal_ctyun == output.role_arn
    error_message = "The shared-bucket policy principal does not match the created IAM role ARN."
  }

  assert {
    condition     = output.object_ds_etag == output.object_etag
    error_message = "The object data source ETag does not match the uploaded object resource."
  }

  assert {
    condition     = output.object_ds_content_length == output.expected_object_length
    error_message = "The object data source content length does not match the uploaded object content."
  }
}

run "verifies_convergence" {
  command = plan
}
