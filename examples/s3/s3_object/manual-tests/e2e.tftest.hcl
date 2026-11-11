# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECT-CLI-01 Create inline and source-backed objects, read all attributes, update content, then confirm no drift.
run "creates_and_reads_object" {
  command = apply

  assert {
    condition     = output.object_id == "${output.object_bucket}/${output.object_key}" && output.object_bucket == output.bucket_name
    error_message = "The object identity or bucket was not read back correctly."
  }

  assert {
    condition     = startswith(output.object_key, "tf-acc-test/s3-object/") && endswith(output.object_key, "/inline.txt") && output.object_content_type == "text/plain"
    error_message = "The object key or content type was not read back correctly."
  }

  assert {
    condition     = output.object_storage_class == "STANDARD"
    error_message = "The object storage class was not read back as STANDARD."
  }

  assert {
    condition     = output.object_metadata.project == "terraform-test" && output.object_metadata.owner == "codex"
    error_message = "The object metadata was not read back correctly."
  }

  assert {
    condition     = output.object_etag != ""
    error_message = "The object ETag was empty after upload."
  }

  assert {
    condition     = output.object_cache_control == null && output.object_content_disposition == null && output.object_content_encoding == null && output.object_content_md5 == null
    error_message = "The inline object's unset request-header attributes were not read back correctly."
  }

  assert {
    condition     = output.object_forbid_overwrite == false && output.object_limit == null && output.object_source == null
    error_message = "The inline object's unset configuration attributes were not preserved correctly."
  }

  assert {
    condition     = output.object_data_location != null && output.object_metadata_location != null && output.object_server_side_encryption != null
    error_message = "The inline object's computed location or encryption output was not available."
  }

  assert {
    condition     = output.source_object_id == "${output.source_object_bucket}/${output.source_object_key}" && output.source_object_bucket == output.bucket_name && startswith(output.source_object_key, "tf-acc-test/s3-object/") && endswith(output.source_object_key, "/source.txt") && output.source_object_arn != ""
    error_message = "The source-backed object's identity attributes were not read back correctly."
  }

  assert {
    condition     = output.source_object_source == "${path.module}/source.txt" && output.source_object_cache_control == "max-age=60" && output.source_object_content_disposition == "attachment; filename=source.txt" && output.source_object_content_encoding == "identity"
    error_message = "The source-backed object's source or request-header attributes were not read back correctly."
  }

  assert {
    condition     = output.source_object_content_md5 == "6m7OQ6rllrMYURIky1gesg==" && output.source_object_content_type == "text/plain" && output.source_object_etag != "" && output.source_object_storage_class == "STANDARD"
    error_message = "The source-backed object's content attributes were not read back correctly."
  }

  assert {
    condition     = output.source_object_forbid_overwrite == true && output.source_object_limit == "rate=1024"
    error_message = "The source-backed object's OOS-specific configuration was not preserved correctly."
  }

  assert {
    condition     = output.source_object_metadata.project == "terraform-test" && output.source_object_metadata.owner == "codex-source" && output.source_object_data_location != null && output.source_object_metadata_location != null && output.source_object_server_side_encryption != null
    error_message = "The source-backed object's metadata, location, or encryption output was not available."
  }
}

run "updates_object_content" {
  command = apply

  variables {
    content = "object content v2"
  }

  assert {
    condition     = output.object_id == "${output.object_bucket}/${output.object_key}" && output.object_etag != ""
    error_message = "Updating object content did not preserve identity or return an ETag."
  }

  assert {
    condition     = output.object_metadata.project == "terraform-test" && output.object_content_type == "text/plain"
    error_message = "Object metadata or content type drifted after the content update."
  }

  assert {
    condition     = output.source_object_id == "${output.source_object_bucket}/${output.source_object_key}" && output.source_object_etag != "" && output.source_object_content_md5 == "6m7OQ6rllrMYURIky1gesg=="
    error_message = "Updating the inline object unexpectedly changed the source-backed object."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    content = "object content v2"
  }

  assert {
    condition     = output.object_id == "${output.object_bucket}/${output.object_key}" && output.source_object_id == "${output.source_object_bucket}/${output.source_object_key}"
    error_message = "Apply was followed by a plan that still expected an object change."
  }
}
