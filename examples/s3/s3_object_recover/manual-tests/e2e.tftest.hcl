# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-RECOVER-CLI-01 Create an object, soft-delete it, recover it by key, then confirm the operation is stable.
run "creates_source_object" {
  command = apply

  assert {
    condition     = output.bucket_name != "" && startswith(output.object_key, "tf-acc-test/s3-object-recover/") && endswith(output.object_key, "/recover.txt")
    error_message = "The source object test did not use the expected bucket and key."
  }
}

run "recovers_soft_deleted_object" {
  command = apply

  variables {
    create_object  = false
    recover_object = true
  }

  assert {
    condition     = output.recover_id != ""
    error_message = "The object recover operation did not produce an operation ID."
  }

  assert {
    condition     = contains(output.recovered_keys, output.object_key)
    error_message = "The recover operation did not report the soft-deleted object as recovered."
  }

  assert {
    condition     = length(output.recover_errors) == 0
    error_message = "The object recover operation returned per-object errors."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    create_object  = false
    recover_object = true
  }

  assert {
    condition     = contains(output.recovered_keys, output.object_key) && length(output.recover_errors) == 0
    error_message = "The recover operation record did not remain stable during plan."
  }
}

# Cleanup: bring the recovered object back under ctyun_oos_object.test management
# (create_object=true) and drop the recover resource (recover_object=false). This
# gives terraform test's automatic teardown a resource that points at the object so
# it gets deleted, preventing recover.txt from lingering as an orphan in the shared bucket.
run "reclaims_recovered_object_for_cleanup" {
  command = apply

  variables {
    create_object  = true
    recover_object = false
  }

  assert {
    condition     = contains(ctyun_oos_object.test[*].key, output.object_key)
    error_message = "The recovered object was not brought back under management for teardown."
  }
}
