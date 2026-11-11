---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_account_password_policy"
description: |-
  Fetch the IAM account password policy.
---

# Data Source: ctyun_iam_account_password_policy

This data source can be used to fetch information about password policy of the OOS account.

## Example Usage

```terraform
data "ctyun_iam_account_password_policy" "example-password-policy" {}

output "minimum_password_length" {
  value = data.ctyun_iam_account_password_policy.example-password-policy.minimum_password_length
}
```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes:

* `allow_users_to_change_password` - Whether to allow IAM users to change the password of the console themselves. Value: `true` (Allow IAM users to change the password of the console themselves), `false` (Not allow IAM users to change the password of the console themselves).
* `expire_passwords` - Whether the password has an expiration time. Value: `true` (There is an expiration time), `false` (There is no expiration time, that is, never expires).
* `hard_expiry` - After the console password expires, whether to allow users to modify the password at the console when they next log in. Value: `true` (After the console password expires, the user cannot log in through the console. It is displayed that the password has expired and the user cannot modify the password through the console), `false` (After the console password expires, the next time the user passes the console, the user directly jumps to the password modification interface).
* `id` - The fixed identifier: `iam-account-password-policy`.
* `max_password_age` - The number of valid days of the password, `0` means never expire.
* `minimum_password_length` - The minimum length of the console password.
* `password_reuse_prevention` - The number of previous passwords that IAM users are prevented from reusing.
* `require_lowercase_characters` - Whether IAM user passwords of console must contain at least one lowercase letter from the ISO basic Latin alphabet (a to z). Value: `true` (IAM user passwords of console must contain at least one lowercase letter), `false` (IAM user passwords of console do not have to contain lowercase letter).
* `require_numbers` - Whether IAM user passwords of console must contain at least one number (0 to 9). Value: `true` (IAM user passwords of console must contain at least one number), `false` (IAM user passwords of console do not have to contain number).
* `require_symbols` - Whether IAM user passwords of console must contain at least one of the following special characters: `!@#$%^&*()_+-=[]{}|`. Value: `true` (IAM user passwords of console must contain at least one special character), `false` (IAM user passwords of console do not have to contain special character).
* `require_uppercase_characters` - Whether IAM user passwords of console must contain at least one uppercase letter from the ISO basic Latin alphabet (A to Z). Value: `true` (IAM user passwords of console must contain at least one uppercase letter), `false` (IAM user passwords of console do not have to contain uppercase letter).
