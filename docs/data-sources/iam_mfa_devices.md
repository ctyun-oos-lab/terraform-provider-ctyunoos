---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_mfa_devices"
description: |-
  Fetch information about the virtual MFA devices associated with the specified IAM user.
---

# Data Source: ctyun_iam_mfa_devices

This data source can be used to fetch information about the virtual MFA devices associated with the specified IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

data "ctyun_iam_mfa_devices" "example-mfa-devices" {
  user_name = ctyun_iam_user.example-user.name
}

output "mfa_devices" {
  value = data.ctyun_iam_mfa_devices.example-mfa-devices.mfa_devices
}
```

## Argument Reference

This data source supports the following arguments:

* `user_name` - (Optional) The name of the IAM user. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`). If no name of the IAM user is specified, it will list all virtual MFA devices associated with the requester.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `mfa_devices` - Information about virtual MFA devices associated with the IAM user. See [`mfa_devices`](#mfa_devices) for details.
* `id` - Region name.

### `mfa_devices`

* `enable_date` - The enabled time of the virtual MFA device.
* `serial_number` - A serial number that uniquely identifies the virtual MFA device.
* `user_name` - The name of the IAM user associated with the virtual MFA device.
