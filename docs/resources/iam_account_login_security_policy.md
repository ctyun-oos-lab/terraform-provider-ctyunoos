---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_account_login_security_policy"
description: |-
  Manages the account login security policy.
---

# Resource: ctyun_iam_account_login_security_policy

Provides an IAM resource to manage the login security policy for IAM users.

## Example Usage

```terraform
resource "ctyun_iam_account_login_security_policy" "example-policy" {
  allow_single_users_simultaneous_login = false
  lockout_duration                      = 30
  login_failed_times                    = 5
  period_with_login_failures            = 30
  session_duration                      = 20
}
```

## Argument Reference

This resource supports the following arguments:

* `allow_single_users_simultaneous_login` - (Optional) Whether to allow IAM users to log in at the same time from different applications. Value: `true` (IAM users are allowed to log in at the same time from different applications), `false` (IAM users are not allowed to log in at the same time from different applications. When the IAM user logs in at the same time from different applications, the last login will be maintained, and the previous login will be forced offline). If this parameter is not specified, the server defaults to `true`.
* `lockout_duration` - (Optional) The time the IAM user was locked out. The value is an integer that ranges from `15` to `60`, the unit is minutes. If this parameter is not specified, the server defaults to `15`.
* `login_failed_times` - (Optional) The number of consecutive login failures allowed for an IAM user within a limited time. The value is an integer that ranges from `5` to `10`. If this parameter is not specified, the server defaults to `5`.
* `period_with_login_failures` - (Optional) The limited time for the number of failed logins. If the IAM user reaches the number of failed logins within the limited login limited time, he will be locked for a period of time. After the lockout period expires, he can log in again. The value is an integer that ranges from `15` to `60`, the unit is minutes. If this parameter is not specified, the server defaults to `15`.
* `session_duration` - (Optional) After the IAM user logs in to the console, the session time is saved when there is no operation. The value is an integer that ranges from `10` to `30`, the unit is minutes. If this parameter is not specified, the server defaults to `30`.

## Attribute Reference

This resource exports the following attribute in addition to the arguments above:

* `id` - The fixed identifier `iam-account-login-security-policy`.

## Import

In Terraform v1.5.0 and later, use an `import` block to import the account login security policy:

```terraform
import {
  to = ctyun_iam_account_login_security_policy.example-policy
  id = "iam-account-login-security-policy"
}
```

Using `terraform import`, import the account login security policy:

```console
terraform import ctyun_iam_account_login_security_policy.example-policy iam-account-login-security-policy
```
