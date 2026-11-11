# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-SOFT-OBJECTS-CLI-01 Create an object, delete it on the next run, and read it from the soft-delete data source.
run "creates_object" {
  command = apply

  assert {
    condition     = output.data_id == output.bucket_name && output.object_key != ""
    error_message = "The soft-delete data source did not use the expected bucket or object key."
  }
}

run "reads_deleted_object" {
  command = apply

  variables {
    create_object = false
  }

  assert {
    condition     = output.data_id == output.bucket_name && output.soft_deleted_count >= 1
    error_message = "The soft-delete data source did not return the object after it was deleted."
  }

  assert {
    condition     = output.soft_deleted_at != ""
    error_message = "The soft-deleted object did not have a soft_delete_at timestamp."
  }

  assert {
    condition     = output.data_max_keys == 1000
    error_message = "The soft-delete data source max_keys was not preserved."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    create_object = false
  }

  assert {
    condition     = output.soft_deleted_count >= 1 && output.soft_deleted_at != ""
    error_message = "Apply was followed by a plan that could not read the soft-deleted object consistently."
  }
}
