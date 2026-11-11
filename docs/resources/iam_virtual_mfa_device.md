---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_virtual_mfa_device"
description: |-
  Provides an IAM virtual MFA device.
---

# Resource: ctyun_iam_virtual_mfa_device

Provides an IAM resource to manage the virtual MFA device.

## Example Usage

```terraform
resource "ctyun_iam_virtual_mfa_device" "example-mfa" {
  virtual_mfa_device_name = "example-mfa"
}

output "qr_code_png" {
  value     = ctyun_iam_virtual_mfa_device.example-mfa.qr_code_png
  sensitive = true
}

output "base_32_string_seed" {
  value     = ctyun_iam_virtual_mfa_device.example-mfa.base_32_string_seed
  sensitive = true
}
```

## Argument Reference

This resource supports the following arguments:

* `virtual_mfa_device_name` - (Required, ForceNew) The name of the virtual MFA device. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The resource of the virtual MFA device. A serial number that uniquely identifies the virtual MFA device.
* `arn` - The resource of the virtual MFA device. A serial number that uniquely identifies the virtual MFA device.
* `base_32_string_seed` - The MFA secret.
* `enable_date` - The enabled time of the virtual MFA device.
* `qr_code_png` - The QR code image of the virtual MFA secret.
* `serial_number` - A serial number that uniquely identifies the virtual MFA device.
* `user_name` - Name of the IAM user associated with this virtual MFA device.

## Import

In Terraform v1.5.0 and later, use an `import` block to import a virtual MFA device by its serial number:

```terraform
import {
  to = ctyun_iam_virtual_mfa_device.example-mfa
  id = "arn:ctyun:iam::1234567890123:mfa/example-mfa"
}
```

Using `terraform import`, import a virtual MFA device by its serial number:

```console
terraform import ctyun_iam_virtual_mfa_device.example-mfa arn:ctyun:iam::1234567890123:mfa/example-mfa
```

The OOS IAM service does not return `base_32_string_seed` or `qr_code_png` during import.
