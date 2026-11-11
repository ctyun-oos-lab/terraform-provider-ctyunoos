# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-COMPOSITE-02 User Role Shared Policy With Users DS
run "creates_and_reads_user_role_shared_policy" {
  command = apply

  assert {
    condition     = output.returned_user_names == output.expected_user_names
    error_message = "The users data source did not return exactly the user created by this test."
  }

  assert {
    condition     = contains(output.user_attached_policy_arns, output.policy_arn)
    error_message = "The user policy attachment data source did not return the shared policy."
  }

  assert {
    condition     = contains(output.role_attached_policy_arns, output.policy_arn)
    error_message = "The role policy attachment data source did not return the shared policy."
  }
}

run "verifies_convergence" {
  command = plan
}
