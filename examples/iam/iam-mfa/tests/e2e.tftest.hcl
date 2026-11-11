# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-MFA-CLI-01 Create an unbound virtual MFA device and read both data sources.
run "creates_and_reads_unbound_virtual_mfa" {
  command = apply

  assert {
    condition     = output.device_serial_number != ""
    error_message = "The virtual MFA device serial_number was empty after apply."
  }

  assert {
    condition     = output.device_id == output.device_serial_number
    error_message = "The virtual MFA device ID should be the serial number."
  }

  assert {
    condition     = can(regex(":mfa/", output.device_arn))
    error_message = "The virtual MFA device ARN does not look like an MFA ARN."
  }

  assert {
    condition     = output.device_user_name == ""
    error_message = "A newly created virtual MFA device should not be bound to a user."
  }

  assert {
    condition     = output.device_enable_date == ""
    error_message = "A newly created virtual MFA device should have an empty enable_date."
  }

  assert {
    condition     = contains(output.virtual_mfa_ds_serials, output.device_serial_number)
    error_message = "The virtual MFA devices data source did not return the created serial number."
  }

  assert {
    condition     = output.virtual_mfa_ds_user_name == ""
    error_message = "The virtual MFA devices data source should report no bound user for an unbound device."
  }

  assert {
    condition     = output.virtual_mfa_ds_enable_date == ""
    error_message = "The virtual MFA devices data source should report an empty enable_date for an unbound device."
  }

  assert {
    condition     = output.mfa_devices_by_user_count == 0
    error_message = "The MFA devices data source should return no devices for a user with no MFA binding."
  }
}

run "verifies_convergence" {
  command = plan

  assert {
    condition     = contains(output.virtual_mfa_ds_serials, output.device_serial_number)
    error_message = "Apply was followed by a plan that still expected an MFA change."
  }
}
