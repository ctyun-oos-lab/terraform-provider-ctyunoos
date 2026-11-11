---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_account_password_policy"
description: |-
  Manages the account password policy.
---

# Resource: ctyun_iam_account_password_policy

Provides an IAM resource to manage password policy of the OOS account.

## Example Usage

```terraform
resource "ctyun_iam_account_password_policy" "example-policy" {
  allow_users_to_change_password = true
  hard_expiry                    = true
  max_password_age               = 90
  minimum_password_length        = 14
  password_reuse_prevention      = 5
  require_lowercase_characters   = true
  require_numbers                = true
  require_symbols                = true
  require_uppercase_characters   = true
}
```

## Argument Reference

This resource supports the following arguments:

* `allow_users_to_change_password` - (Optional) Whether to allow IAM users to change the password of the console themselves. Value: `true` (Allow IAM users to change the password of the console themselves), `false` (Not allow IAM users to change the password of the console themselves). If this parameter is not specified, the server defaults to `true`.
* `hard_expiry` - (Optional) After the console password expires, whether to allow users to modify the password at the console when they next log in. Value: `true` (After the console password expires, the user cannot log in through the console. It is displayed that the password has expired and the user cannot modify the password through the console), `false` (After the console password expires, the next time the user passes the console, the user directly jumps to the password modification interface). If this parameter is not specified, the server defaults to `false`.
* `max_password_age` - (Optional) The number of days that an IAM user password is valid. The value is `0` to `1095`, the unit is day. `0` means never expire. If this parameter is not specified, the server defaults to `0`.
* `minimum_password_length` - (Optional) The minimum length of the console password. The value is `8` to `128`. If this parameter is not specified, the server defaults to `8`.
* `password_reuse_prevention` - (Optional) Specifies the number of previous passwords that IAM users are prevented from reusing. The value is `0` to `24`, `0` means that the IAM user can reuse the previous passwords. If this parameter is not specified, the server defaults to `0`. 
* `require_lowercase_characters` - (Optional) Specifies whether IAM user passwords of console must contain at least one lowercase letter from the ISO basic Latin alphabet (`a` to `z`). Value: `true` (IAM user passwords of console must contain at least one lowercase letter), `false` (IAM user passwords of console do not have to contain lowercase letter). If this parameter is not specified, the server defaults to `true`.
* `require_numbers` - (Optional) Specifies whether IAM user passwords of console must contain at least one number (`0` to `9`). Value: `true` (IAM user passwords of console must contain at least one number). `false` (IAM user passwords of console do not have to contain number). If this parameter is not specified, the server defaults to `true`.
* `require_symbols` - (Optional) Specifies whether IAM user passwords of console must contain at least one of the following special characters: `!@#$%^&*`. Value: `true` (IAM user passwords of console must contain at least one special character), `false` (IAM user passwords of console do not have to contain special character). If this parameter is not specified, the server defaults to `false`.
* `require_uppercase_characters` - (Optional) Specifies whether IAM user passwords of console must contain at least one uppercase letter from the ISO basic Latin alphabet (`A` to `Z`). Value: `true` (IAM user passwords of console must contain at least one uppercase letter), `false` (IAM user passwords of console do not have to contain uppercase letter). If this parameter is not specified, the server defaults to `false`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The fixed identifier `iam-account-password-policy`.
* `expire_passwords` - Indicates whether passwords in the account expire. Value: `true` (max_password_age is greater than `0`), `false` (max_password_age is `0` or not set).

## Import

In Terraform v1.5.0 and later, use an `import` block to import the account password policy:

```terraform
import {
  to = ctyun_iam_account_password_policy.example-policy
  id = "iam-account-password-policy"
}
```

Using `terraform import`, import the account password policy:

```console
terraform import ctyun_iam_account_password_policy.example-policy iam-account-password-policy
```
