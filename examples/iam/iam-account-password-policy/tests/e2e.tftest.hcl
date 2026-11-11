# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-APP-CLI-01 Create the account password policy, read it through the data source, update, then confirm no drift.
run "creates_and_reads_policy" {
  command = apply

  assert {
    condition     = output.policy_id == "iam-account-password-policy"
    error_message = "The password policy resource ID should be iam-account-password-policy."
  }

  assert {
    condition     = output.data_policy_id == output.policy_id
    error_message = "The password policy data source ID does not match the resource."
  }

  assert {
    condition     = output.allow_users_to_change_password == false
    error_message = "allow_users_to_change_password was not written back as false."
  }

  assert {
    condition     = output.hard_expiry == true
    error_message = "hard_expiry was not written back as true."
  }

  assert {
    condition     = output.max_password_age == 90
    error_message = "max_password_age was not written back as 90."
  }

  assert {
    condition     = output.expire_passwords == true
    error_message = "expire_passwords should be true when max_password_age is greater than 0."
  }

  assert {
    condition     = output.minimum_password_length == 16
    error_message = "minimum_password_length was not written back as 16."
  }

  assert {
    condition     = output.password_reuse_prevention == 5
    error_message = "password_reuse_prevention was not written back as 5."
  }

  assert {
    condition     = output.require_lowercase_characters == false
    error_message = "require_lowercase_characters was not written back as false."
  }

  assert {
    condition     = output.require_numbers == false
    error_message = "require_numbers was not written back as false."
  }

  assert {
    condition     = output.require_symbols == true
    error_message = "require_symbols was not written back as true."
  }

  assert {
    condition     = output.require_uppercase_characters == true
    error_message = "require_uppercase_characters was not written back as true."
  }

  assert {
    condition     = output.data_allow_users_to_change_password == output.allow_users_to_change_password
    error_message = "The data source allow_users_to_change_password does not match the resource."
  }

  assert {
    condition     = output.data_expire_passwords == output.expire_passwords
    error_message = "The data source expire_passwords does not match the resource."
  }

  assert {
    condition     = output.data_hard_expiry == output.hard_expiry
    error_message = "The data source hard_expiry does not match the resource."
  }

  assert {
    condition     = output.data_max_password_age == output.max_password_age
    error_message = "The data source max_password_age does not match the resource."
  }

  assert {
    condition     = output.data_minimum_password_length == output.minimum_password_length
    error_message = "The data source minimum_password_length does not match the resource."
  }

  assert {
    condition     = output.data_password_reuse_prevention == output.password_reuse_prevention
    error_message = "The data source password_reuse_prevention does not match the resource."
  }

  assert {
    condition     = output.data_require_lowercase_characters == output.require_lowercase_characters
    error_message = "The data source require_lowercase_characters does not match the resource."
  }

  assert {
    condition     = output.data_require_numbers == output.require_numbers
    error_message = "The data source require_numbers does not match the resource."
  }

  assert {
    condition     = output.data_require_symbols == output.require_symbols
    error_message = "The data source require_symbols does not match the resource."
  }

  assert {
    condition     = output.data_require_uppercase_characters == output.require_uppercase_characters
    error_message = "The data source require_uppercase_characters does not match the resource."
  }
}

# After Update, the managed resource can keep the previous computed
# expire_passwords until the next refresh. The data source re-reads
# GetAccountPasswordPolicy in this apply; the resource is checked in
# verifies_convergence.
run "updates_policy" {
  command = apply

  variables {
    allow_users_to_change_password = true
    hard_expiry                    = false
    max_password_age               = 0
    minimum_password_length        = 9
    password_reuse_prevention      = 3
    require_lowercase_characters   = true
    require_numbers                = true
    require_symbols                = false
    require_uppercase_characters   = false
  }

  assert {
    condition     = output.allow_users_to_change_password == true
    error_message = "Updating allow_users_to_change_password did not read back true."
  }

  assert {
    condition     = output.hard_expiry == false
    error_message = "Updating hard_expiry did not read back false."
  }

  assert {
    condition     = output.max_password_age == 0
    error_message = "Updating max_password_age did not read back 0."
  }

  assert {
    condition     = output.minimum_password_length == 9
    error_message = "Updating minimum_password_length did not read back 9."
  }

  assert {
    condition     = output.password_reuse_prevention == 3
    error_message = "Updating password_reuse_prevention did not read back 3."
  }

  assert {
    condition     = output.require_lowercase_characters == true && output.require_numbers == true
    error_message = "Updating require_lowercase_characters or require_numbers did not read back true."
  }

  assert {
    condition     = output.require_symbols == false && output.require_uppercase_characters == false
    error_message = "Updating require_symbols or require_uppercase_characters did not read back false."
  }

  assert {
    condition     = output.data_expire_passwords == false
    error_message = "The data source expire_passwords drifted after the update."
  }

  assert {
    condition     = output.data_minimum_password_length == 9
    error_message = "The data source minimum_password_length drifted after the update."
  }

  assert {
    condition     = output.data_password_reuse_prevention == 3
    error_message = "The data source password_reuse_prevention drifted after the update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    allow_users_to_change_password = true
    hard_expiry                    = false
    max_password_age               = 0
    minimum_password_length        = 9
    password_reuse_prevention      = 3
    require_lowercase_characters   = true
    require_numbers                = true
    require_symbols                = false
    require_uppercase_characters   = false
  }

  assert {
    condition     = output.minimum_password_length == 9
    error_message = "Apply was followed by a plan that still expected a password policy change."
  }

  assert {
    condition     = output.expire_passwords == false
    error_message = "expire_passwords should be false after refresh when max_password_age is 0."
  }

  assert {
    condition     = output.data_expire_passwords == false
    error_message = "The data source expire_passwords should stay false after the follow-up plan."
  }
}
