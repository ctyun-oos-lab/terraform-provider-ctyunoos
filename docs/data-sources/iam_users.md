---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_users"
description: |-
  Fetch information about specified IAM users or all IAM users.
---

# Data Source: ctyun_iam_users

This data source can be used to fetch information about specified IAM users or all IAM users.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

data "ctyun_iam_users" "example-users" {
  name_regex = ctyun_iam_user.example-user.name
}

output "users_name_0" {
  value = data.ctyun_iam_users.example-users.users.0.user_name
}
```

## Argument Reference

This data source supports the following arguments:

* `name_regex` - (Optional) A regular expression used to filter by name.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arns` - Resource names of matched IAM users.
* `id` - Region name.
* `names` - Names of matched users.
* `users` - Information about IAM users. See [`users`](#users) for details.

### `users`

* `arn` - Resource name of the IAM user.
* `create_date` - Creation time of the IAM user.
* `password_last_used` - The last time that the user used the user password to log in to the OOS website.
* `user_id` - ID of the IAM user.
* `user_name` - Name of the IAM user.
