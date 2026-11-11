# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COMPOSITE-01 Object Data Sources With Shared Bucket
run "creates_and_reads_shared_bucket_object" {
  command = apply

  assert {
    condition     = output.objects_ds_keys == toset([output.expected_object_key])
    error_message = "The objects data source prefix query did not return exactly the uploaded object key."
  }

  assert {
    condition     = output.object_ds_etag == output.object_etag
    error_message = "The object data source etag does not match the uploaded object resource."
  }

  assert {
    condition     = output.object_ds_content_length == output.expected_object_length
    error_message = "The object data source content length does not match the uploaded object content."
  }
}

run "verifies_convergence" {
  command = plan
}
