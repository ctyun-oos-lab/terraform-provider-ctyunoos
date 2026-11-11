# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SSE-CLI-01 Create a server-side encryption configuration, read it through the data source, then confirm no drift.
run "creates_and_reads_configuration" {
  command = apply

  assert {
    condition     = output.resource_id == output.bucket_name && output.data_id == output.bucket_name
    error_message = "The encryption resource and data source IDs should be the bucket name."
  }

  assert {
    condition     = output.resource_rule_count == 1 && output.data_rule_count == 1
    error_message = "The resource or data source did not return exactly one encryption rule."
  }

  assert {
    condition     = output.resource_sse_algorithm == "AES256" && output.data_sse_algorithm == output.resource_sse_algorithm
    error_message = "The server-side encryption algorithm was not read back as AES256 by both paths."
  }
}

run "verifies_convergence" {
  command = plan

  assert {
    condition     = output.resource_id == output.bucket_name && output.resource_sse_algorithm == "AES256"
    error_message = "Apply was followed by a plan that still expected a server-side encryption change."
  }

  assert {
    condition     = output.data_id == output.bucket_name && output.data_sse_algorithm == "AES256"
    error_message = "The encryption data source did not remain consistent during plan."
  }
}
