# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COMPOSITE-01 Full Inline Bucket
run "creates_and_reads_full_inline_bucket" {
  command = apply

  assert {
    condition     = output.inline_encryption_algorithm == "AES256"
    error_message = "The main bucket did not read back its inline AES256 encryption configuration."
  }

  assert {
    condition     = output.inline_lifecycle_days == 30
    error_message = "The main bucket did not read back its inline 30-day expiration rule."
  }

  assert {
    condition     = output.bucket_policy_bucket == output.bucket_name
    error_message = "The bucket policy does not reference the main bucket."
  }

  assert {
    condition     = output.inline_logging_target_bucket == output.log_bucket_name
    error_message = "The inline logging target bucket does not match the independent log bucket."
  }
}

run "verifies_convergence" {
  command = plan
}
