---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_virtual_mfa_devices"
description: |-
  Fetch information about virtual MFA devices within OOS account or IAM user.
---

# Data Source: ctyun_iam_virtual_mfa_devices

This data source can be used to fetch information about virtual MFA devices within OOS account or IAM user.

## Example Usage

```terraform
resource "ctyun_iam_virtual_mfa_device" "example-mfa-device" {
  virtual_mfa_device_name = "example-mfa-device"
}

data "ctyun_iam_virtual_mfa_devices" "example-mfa-devices" {
  depends_on = [ctyun_iam_virtual_mfa_device.example-mfa-device]
}

output "virtual_mfa_serial_number_0" {
  value = data.ctyun_iam_virtual_mfa_devices.example-mfa-devices.virtual_mfa_devices.0.serial_number
}
```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes:

* `virtual_mfa_devices` - Information about virtual MFA devices. See [`virtual_mfa_devices`](#virtual_mfa_devices) for details.
* `id` - Region name.

### `virtual_mfa_devices`

* `enable_date` - The enabled time of the virtual MFA device.
* `serial_number` - A serial number that uniquely identifies the virtual MFA device.
* `user` - Information about IAM user associated with this virtual MFA device.

The `user` configuration block supports the following argument:

* `arn` - Resource name of the IAM user.
* `create_date` - Creation time of the IAM user.
* `password_last_used` - The last time that the user used the user password to log in to the OOS website.
* `user_id` - ID of the IAM user.
* `user_name` - Name of the IAM user.
