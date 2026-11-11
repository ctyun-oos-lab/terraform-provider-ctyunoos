# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COMPOSITE-03 Multiple Objects With Objects DS
run "creates_and_lists_multiple_objects" {
  command = apply

  assert {
    condition     = alltrue([for object_key in output.expected_object_keys : contains(output.returned_object_keys, object_key)])
    error_message = "The objects data source did not return both count-created object keys."
  }

  assert {
    condition     = output.object_buckets == toset([output.bucket_name])
    error_message = "The count-created objects did not all reference the shared test bucket."
  }
}

run "verifies_convergence" {
  command = plan
}
