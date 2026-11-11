# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COMPOSITE-02 Object Copy And Data Source
run "creates_and_reads_object_copy" {
  command = apply

  assert {
    condition     = output.copy_source == output.expected_copy_source && output.copy_bucket == output.source_bucket
    error_message = "The object copy source is not formatted as {bucket}/{source-key}."
  }

  assert {
    condition     = output.source_key != output.target_key && output.bucket_name != ""
    error_message = "The object copy did not use a distinct target key in the shared bucket."
  }

  assert {
    condition     = output.copy_etag == output.source_etag && output.object_ds_etag == output.source_etag
    error_message = "The copied object and object data source ETags do not match the source object."
  }
}

run "verifies_convergence" {
  command = plan
}
