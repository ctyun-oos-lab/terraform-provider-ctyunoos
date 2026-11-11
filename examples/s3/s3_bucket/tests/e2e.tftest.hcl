# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-BUCKET-CLI-01 Create a bucket, update its ACL, then confirm no drift.
run "creates_and_reads_bucket" {
  command = apply

  assert {
    condition     = output.bucket_id == output.bucket && output.bucket_arn == "arn:ctyun:oos:::${output.bucket}"
    error_message = "The bucket ID or ARN was not read back correctly."
  }

  assert {
    condition     = output.bucket_acl == "private" && output.bucket_force_destroy == true
    error_message = "The bucket ACL or force_destroy setting was not read back correctly."
  }

  assert {
    condition     = output.bucket_lifecycle_rule_count == 1 && output.bucket_logging_target != ""
    error_message = "The inline lifecycle or logging configuration was not read back correctly."
  }

  assert {
    condition     = output.bucket_object_lock_enabled == "Disabled" && output.bucket_object_lock_days == 30 && output.bucket_sse_algorithm == "AES256"
    error_message = "The inline Object Lock or encryption configuration was not read back correctly."
  }

  assert {
    condition     = output.bucket_policy != "" && output.bucket_policy_action == "oos:GetObject" && output.bucket_policy_effect == "Allow"
    error_message = "The inline bucket policy inputs were not read back correctly."
  }
}

run "updates_object_lock_configuration" {
  command = apply

  variables {
    object_lock_days = 60
  }

  assert {
    condition     = output.bucket_object_lock_enabled == "Disabled" && output.bucket_object_lock_days == 60
    error_message = "Updating the inline Object Lock retention period did not read back correctly."
  }
}

run "updates_policy_inputs" {
  command = apply

  variables {
    policy_action = "oos:ListBucket"
    policy_effect = "Deny"
  }

  assert {
    condition     = output.bucket_policy_action == "oos:ListBucket" && output.bucket_policy_effect == "Deny" && output.bucket_policy != ""
    error_message = "Updating inline bucket policy inputs did not read back correctly."
  }
}

run "updates_bucket_acl" {
  command = apply

  variables {
    acl              = "public-read"
    object_lock_days = 60
    policy_action    = "oos:ListBucket"
    policy_effect    = "Deny"
  }

  assert {
    condition     = output.bucket_acl == "public-read" && output.bucket_id == output.bucket
    error_message = "Updating the bucket ACL did not read back public-read or changed identity."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    acl              = "public-read"
    object_lock_days = 60
    policy_action    = "oos:ListBucket"
    policy_effect    = "Deny"
  }

  assert {
    condition     = output.bucket_acl == "public-read" && output.bucket_force_destroy == true && output.bucket_object_lock_days == 60 && output.bucket_policy_action == "oos:ListBucket" && output.bucket_policy_effect == "Deny"
    error_message = "Apply was followed by a plan that still expected a bucket change."
  }
}
