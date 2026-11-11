---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_account_summary"
description: |-
  Fetch IAM account resource counts and quotas.
---

# Data Source: ctyun_iam_account_summary

This data source can be used to fetch IAM account resource counts and quotas.

## Example Usage

```terraform
data "ctyun_iam_account_summary" "example-summary" {}

output "users" {
  value = data.ctyun_iam_account_summary.example-summary.users
}
```

## Argument Reference

This data source does not support any arguments.

## Attribute Reference

This data source exports the following attributes:

* `id` - Fixed identifier of this data source: `iam_account_summary`.
* `accesskeys_per_user_quota` - The maximum number of AccessKeys per IAM user.
* `account_accesskeys_present` - The number of AccessKeys already in the OOS account.
* `attached_policies_per_group_quota` - The maximum number of policies attached to each IAM group.
* `attached_policies_per_role_quota` - The maximum number of policies attached to each IAM role.
* `attached_policies_per_user_quota` - The maximum number of policies attached to each IAM user.
* `groups` - The number of groups already in the OOS account.
* `groups_per_user_quota` - The maximum number of groups that an IAM user can belong to.
* `groups_quota` - The maximum number of IAM groups in OOS account.
* `mfa_devices` - The number of virtual MFA devices already in the OOS account.
* `mfa_devices_inuse` - The number of virtual MFA devices which are enabled in the OOS account.
* `policies` - The number of policies already in the OOS account.
* `policies_quota` - The maximum number of policies in the OOS account.
* `providers` - The number of SAML 2.0 providers already in the OOS account.
* `providers_quota` - The maximum number of SAML 2.0 providers in the OOS account.
* `roles` - The number of IAM roles already in the OOS account.
* `roles_quota` - The maximum number of IAM roles in the OOS account.
* `users` - The number of IAM users already in the OOS account.
* `users_quota` - The maximum number of IAM users in the OOS account.
