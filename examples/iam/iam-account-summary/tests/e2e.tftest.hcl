# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-DAS-CLI-01 Read the account summary, then create one user/group/policy/role and confirm counters increase.
run "reads_account_summary" {
  command = apply

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "The account summary data source ID should be iam_account_summary."
  }

  assert {
    condition     = output.users >= 0 && output.groups >= 0 && output.policies >= 0 && output.roles >= 0
    error_message = "Account usage counters should be non-negative."
  }

  assert {
    condition     = output.mfa_devices >= 0 && output.mfa_devices_inuse >= 0 && output.mfa_devices_inuse <= output.mfa_devices
    error_message = "MFA device counters were invalid."
  }

  assert {
    condition     = output.providers >= 0 && output.account_accesskeys_present >= 0
    error_message = "Provider or account access key counters were negative."
  }

  assert {
    condition = (
      output.users_quota > 0 &&
      output.groups_quota > 0 &&
      output.policies_quota > 0 &&
      output.roles_quota > 0 &&
      output.providers_quota > 0 &&
      output.accesskeys_per_user_quota > 0 &&
      output.groups_per_user_quota > 0 &&
      output.attached_policies_per_user_quota > 0 &&
      output.attached_policies_per_group_quota > 0 &&
      output.attached_policies_per_role_quota > 0
    )
    error_message = "Account quota fields should be greater than zero."
  }

  assert {
    condition = (
      output.users <= output.users_quota &&
      output.groups <= output.groups_quota &&
      output.policies <= output.policies_quota &&
      output.roles <= output.roles_quota &&
      output.providers <= output.providers_quota
    )
    error_message = "A usage counter exceeded its quota."
  }
}

run "counts_increase_after_create" {
  command = apply

  variables {
    create_resources = true
  }

  assert {
    condition     = output.users_delta == 1
    error_message = "Creating one IAM user did not increase the users counter by 1."
  }

  assert {
    condition     = output.groups_delta == 1
    error_message = "Creating one IAM group did not increase the groups counter by 1."
  }

  assert {
    condition     = output.policies_delta == 1
    error_message = "Creating one IAM policy did not increase the policies counter by 1."
  }

  assert {
    condition     = output.roles_delta == 1
    error_message = "Creating one IAM role did not increase the roles counter by 1."
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "The account summary data source ID drifted after creating resources."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    create_resources = true
  }

  assert {
    condition     = output.users_delta == 0
    error_message = "The follow-up plan should not detect any further account summary change."
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Apply was followed by a plan that still expected an account summary change."
  }
}
