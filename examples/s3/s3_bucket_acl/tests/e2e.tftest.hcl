# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-ACL-CLI-01 Read and update a bucket canned ACL, then restore private ACL and confirm convergence.
run "creates_and_reads_private_acl" {
  command = apply

  assert {
    condition     = output.bucket == output.bucket_created && output.acl == "private"
    error_message = "The bucket ACL resource did not read back the expected bucket and private ACL."
  }

  assert {
    condition     = output.access_control_policy_count == 1 && output.owner_count == 1 && output.grant_count >= 1
    error_message = "The bucket ACL access_control_policy was not expanded correctly."
  }
}

run "updates_to_public_read" {
  command = apply

  variables {
    acl = "public-read"
  }

  assert {
    condition     = output.acl == "public-read" && output.bucket == output.bucket_created
    error_message = "Updating the bucket ACL did not read back public-read."
  }
}

run "restores_private_acl" {
  command = apply

  variables {
    acl = "private"
  }

  assert {
    condition     = output.acl == "private" && output.grant_count >= 1
    error_message = "Restoring the bucket ACL to private did not read back correctly."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    acl = "private"
  }

  assert {
    condition     = output.acl == "private" && output.bucket == output.bucket_created
    error_message = "Apply was followed by a plan that still expected a bucket ACL change."
  }
}
