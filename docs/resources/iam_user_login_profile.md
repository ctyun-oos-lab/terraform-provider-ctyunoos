---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user_login_profile"
description: |-
  Manages an IAM user login profile.
---

# Resource: ctyun_iam_user_login_profile

Provides an IAM resource to manage the console login password for the specified IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

resource "ctyun_iam_user_login_profile" "example-profile" {
  user                    = ctyun_iam_user.example-user.name
  password_reset_required = true
}

output "initial_password" {
  value     = ctyun_iam_user_login_profile.example-profile.password
  sensitive = true
}
```

## Argument Reference

This resource supports the following arguments:

* `user` - (Required, ForceNew) The name of the IAM user. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `password_reset_required` - (Optional, ForceNew) Specifies whether the IAM user is required to reset a new password after logging in to the console with the initial password. Value: `true` (After logging in with the initial password, you cannot perform the operation without resetting the password), `false` (After logging in with the initial password, you can perform the operation without resetting the password). If this parameter is not specified, the server defaults to `false`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the IAM user.
* `password` - The console login password.

## Import

In Terraform v1.5.0 and later, use an `import` block to import an IAM user login profile by user name:

```terraform
import {
  to = ctyun_iam_user_login_profile.example-profile
  id = "example-user"
}
```

Using `terraform import`, import an IAM user login profile by user name:

```console
terraform import ctyun_iam_user_login_profile.example-profile example-user
```

The generated password cannot be read during import.
