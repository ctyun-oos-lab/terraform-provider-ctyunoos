# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-COPY-CLI-01 Create two copies, including conditional copy headers, update copy metadata/storage, then confirm no drift.
run "creates_and_reads_copy" {
  command = apply

  assert {
    condition     = output.source_id == "${output.source_bucket}/${output.source_key}" && output.source_bucket == output.bucket_name && startswith(output.source_key, "tf-acc-test/s3-object-copy/") && endswith(output.source_key, "/source.txt")
    error_message = "The source object was not created in the expected bucket and key."
  }

  assert {
    condition     = output.copy_id == output.copy_key && output.copy_bucket == output.bucket_name && startswith(output.copy_key, "tf-acc-test/s3-object-copy/") && endswith(output.copy_key, "/target.txt")
    error_message = "The copied object identity or key was not read back correctly."
  }

  assert {
    condition     = output.copy_source == output.source_id && output.copy_arn != ""
    error_message = "The copied object source or ARN was not preserved."
  }

  assert {
    condition     = output.copy_content_type == "text/plain" && output.copy_storage_class == "STANDARD"
    error_message = "The copied object content type or storage class was not read back correctly."
  }

  assert {
    condition     = output.copy_metadata.project == "terraform-test" && output.copy_metadata.owner == "team-a" && output.copy_etag != ""
    error_message = "The copied object metadata or ETag was not read back correctly."
  }

  assert {
    condition     = output.copy_cache_control == "" && output.copy_content_disposition == "" && output.copy_content_encoding == "" && output.copy_metadata_directive == "REPLACE"
    error_message = "The primary copy's optional headers or metadata directive were not read back correctly."
  }

  assert {
    condition     = can(output.copy_data_location) && can(output.copy_expiration) && can(output.copy_last_modified) && can(output.copy_metadata_location) && can(output.copy_server_side_encryption)
    error_message = "The primary copy's computed outputs were not populated correctly."
  }

  assert {
    condition     = output.headers_copy_id == output.headers_copy_key && output.headers_copy_bucket == output.bucket_name && startswith(output.headers_copy_key, "tf-acc-test/s3-object-copy/") && endswith(output.headers_copy_key, "/target-headers.txt") && output.headers_copy_arn != ""
    error_message = "The conditional copy identity outputs were not read back correctly."
  }

  assert {
    condition     = output.headers_copy_source == output.source_id && output.headers_copy_cache_control == "max-age=60" && output.headers_copy_content_disposition == "attachment; filename=copy-target-headers.txt" && output.headers_copy_content_encoding == "identity" && output.headers_copy_content_type == "text/plain"
    error_message = "The conditional copy request-header outputs were not read back correctly."
  }

  assert {
    condition     = output.headers_copy_copy_if_match != "" && output.headers_copy_copy_if_modified_since == "2000-01-01T00:00:00Z" && output.headers_copy_copy_if_none_match == "non-matching-etag" && output.headers_copy_copy_if_unmodified_since == "2100-01-01T00:00:00Z"
    error_message = "The conditional copy inputs were not preserved in state."
  }

  assert {
    condition     = output.headers_copy_metadata_directive == "REPLACE" && output.headers_copy_metadata.source == "conditional-copy" && output.headers_copy_etag != "" && output.headers_copy_storage_class == "STANDARD" && can(output.headers_copy_data_location) && can(output.headers_copy_expiration) && can(output.headers_copy_last_modified) && can(output.headers_copy_metadata_location) && can(output.headers_copy_server_side_encryption)
    error_message = "The conditional copy computed outputs or metadata directive were not read back correctly."
  }
}

run "updates_copy_metadata_and_storage" {
  command = apply

  variables {
    target_storage_class = "STANDARD"
    target_owner         = "team-b"
  }

  assert {
    condition     = output.copy_storage_class == "STANDARD" && output.copy_metadata.owner == "team-b"
    error_message = "Updating the copied object storage class or metadata did not read back the new values."
  }

  assert {
    condition     = output.copy_id == output.copy_key && output.copy_content_type == "text/plain"
    error_message = "The copied object identity or content type changed unexpectedly during update."
  }

  assert {
    condition     = output.headers_copy_id == output.headers_copy_key && output.headers_copy_etag != "" && output.headers_copy_copy_if_none_match == "non-matching-etag"
    error_message = "Updating the primary copy unexpectedly changed the conditional copy."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    target_storage_class = "STANDARD"
    target_owner         = "team-b"
  }

  assert {
    condition     = output.copy_storage_class == "STANDARD" && output.copy_metadata.owner == "team-b" && output.headers_copy_storage_class == "STANDARD"
    error_message = "Apply was followed by a plan that still expected an object copy change."
  }
}
