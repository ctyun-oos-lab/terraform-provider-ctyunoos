# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-COMPOSITE-01 Full IAM Chain With Users DS
run "creates_and_reads_full_iam_chain" {
  command = apply

  assert {
    condition     = output.returned_user_names == output.expected_user_names
    error_message = "The users data source did not return exactly the two users created by this test."
  }

  assert {
    condition     = alltrue([for group_names in values(output.group_names_by_user) : contains(group_names, output.group_name)])
    error_message = "The user-group membership data source did not return the shared group for every user."
  }

  assert {
    condition     = alltrue([for policy_arns in values(output.attached_policy_arns_by_user) : contains(policy_arns, output.policy_arn)])
    error_message = "The user policy attachment data source did not return the shared policy for every user."
  }
}

run "verifies_convergence" {
  command = plan
}
