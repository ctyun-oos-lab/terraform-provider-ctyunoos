---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_account_login_security_policy"
description: |-
  Fetch the IAM account login security policy.
---

# Data Source: ctyun_iam_account_login_security_policy

This data source can be used to fetch information about the login security policy for IAM users.

## Example Usage

```terraform
data "ctyun_iam_account_login_security_policy" "example-login-security-policy" {}

output "session_duration" {
  value = data.ctyun_iam_account_login_security_policy.example-login-security-policy.session_duration
}
```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes:

* `allow_single_users_simultaneous_login` - Whether to allow IAM users to log in at the same time from different applications. Value: `true` (IAM users are allowed to log in at the same time from different applications), `false` (IAM users are not allowed to log in at the same time from different applications. When the IAM user logs in at the same time from different applications, the last login will be maintained, and the previous login will be forced offline).
* `id` - Fixed identifier of this data source: `iam-account-login-security-policy`.
* `lockout_duration` - The time the IAM user was locked out.
* `login_failed_times` - The number of consecutive login failures allowed for an IAM user within a limited time.
* `period_with_login_failures` - The limited time for the number of failed logins. If the IAM user reaches the number of failed logins within the limited login limited time, he will be locked for a period of time. After the lockout period expires, he can log in again.
* `session_duration` - After the IAM user logs in to the console, the session time is saved when there is no operation.
