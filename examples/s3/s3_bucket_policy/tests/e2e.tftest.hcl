# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-POLICY-CLI-01 Create a bucket policy, update its statement, then confirm no drift.
run "creates_and_reads_policy" {
  command = apply

  assert {
    condition     = output.policy_id == output.bucket_name
    error_message = "The bucket policy resource ID should be the bucket name."
  }

  assert {
    condition     = output.policy_version == "2012-10-17"
    error_message = "The policy Version was not read back correctly."
  }

  assert {
    condition     = output.statement_effect == "Allow"
    error_message = "The policy statement Effect was not read back as Allow."
  }

  assert {
    condition     = output.statement_action == "oos:*"
    error_message = "The policy statement Action was not read back as oos:*."
  }

  assert {
    condition     = output.statement_resource == "arn:ctyun:oos:::${output.bucket_name}/*"
    error_message = "The policy statement Resource did not target the created bucket."
  }
}

run "updates_policy_action" {
  command = apply

  variables {
    action = "oos:GetObject"
  }

  assert {
    condition     = output.statement_action == "oos:GetObject"
    error_message = "Updating the policy Action did not read back oos:GetObject."
  }

  assert {
    condition     = output.policy_id == output.bucket_name && output.statement_effect == "Allow"
    error_message = "The bucket policy identity or Effect changed unexpectedly during update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    action = "oos:GetObject"
  }

  assert {
    condition     = output.statement_action == "oos:GetObject"
    error_message = "Apply was followed by a plan that still expected a bucket policy change."
  }
}
