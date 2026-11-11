# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-ALSP-CLI-01 Create the account login security policy, read it through the data source, update, then confirm no drift.
run "creates_and_reads_policy" {
  command = apply

  assert {
    condition     = output.policy_id == "iam-account-login-security-policy"
    error_message = "The login security policy resource ID should be iam-account-login-security-policy."
  }

  assert {
    condition     = output.data_policy_id == output.policy_id
    error_message = "The login security policy data source ID does not match the resource."
  }

  assert {
    condition     = output.allow_single_users_simultaneous_login == false
    error_message = "allow_single_users_simultaneous_login was not written back as false."
  }

  assert {
    condition     = output.lockout_duration == 20
    error_message = "lockout_duration was not written back as 20."
  }

  assert {
    condition     = output.login_failed_times == 7
    error_message = "login_failed_times was not written back as 7."
  }

  assert {
    condition     = output.period_with_login_failures == 25
    error_message = "period_with_login_failures was not written back as 25."
  }

  assert {
    condition     = output.session_duration == 25
    error_message = "session_duration was not written back as 25."
  }

  assert {
    condition     = output.data_allow_single_users_simultaneous_login == output.allow_single_users_simultaneous_login
    error_message = "The data source allow_single_users_simultaneous_login does not match the resource."
  }

  assert {
    condition     = output.data_lockout_duration == output.lockout_duration
    error_message = "The data source lockout_duration does not match the resource."
  }

  assert {
    condition     = output.data_login_failed_times == output.login_failed_times
    error_message = "The data source login_failed_times does not match the resource."
  }

  assert {
    condition     = output.data_period_with_login_failures == output.period_with_login_failures
    error_message = "The data source period_with_login_failures does not match the resource."
  }

  assert {
    condition     = output.data_session_duration == output.session_duration
    error_message = "The data source session_duration does not match the resource."
  }
}

run "updates_policy" {
  command = apply

  variables {
    allow_single_users_simultaneous_login = true
    lockout_duration                      = 30
    login_failed_times                    = 8
    period_with_login_failures            = 40
    session_duration                      = 15
  }

  assert {
    condition     = output.allow_single_users_simultaneous_login == true
    error_message = "Updating allow_single_users_simultaneous_login did not read back true."
  }

  assert {
    condition     = output.lockout_duration == 30
    error_message = "Updating lockout_duration did not read back 30."
  }

  assert {
    condition     = output.login_failed_times == 8
    error_message = "Updating login_failed_times did not read back 8."
  }

  assert {
    condition     = output.period_with_login_failures == 40
    error_message = "Updating period_with_login_failures did not read back 40."
  }

  assert {
    condition     = output.session_duration == 15
    error_message = "Updating session_duration did not read back 15."
  }

  assert {
    condition     = output.data_allow_single_users_simultaneous_login == true
    error_message = "The data source allow_single_users_simultaneous_login drifted after the update."
  }

  assert {
    condition     = output.data_lockout_duration == 30
    error_message = "The data source lockout_duration drifted after the update."
  }

  assert {
    condition     = output.data_login_failed_times == 8
    error_message = "The data source login_failed_times drifted after the update."
  }

  assert {
    condition     = output.data_period_with_login_failures == 40
    error_message = "The data source period_with_login_failures drifted after the update."
  }

  assert {
    condition     = output.data_session_duration == 15
    error_message = "The data source session_duration drifted after the update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    allow_single_users_simultaneous_login = true
    lockout_duration                      = 30
    login_failed_times                    = 8
    period_with_login_failures            = 40
    session_duration                      = 15
  }

  assert {
    condition     = output.session_duration == 15
    error_message = "Apply was followed by a plan that still expected a login security policy change."
  }
}
