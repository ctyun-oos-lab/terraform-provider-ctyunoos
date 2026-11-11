---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_group"
description: |-
  Fetch information on an IAM group.
---

# Data Source: ctyun_iam_group

This data source can be used to fetch information about the specific IAM group.

## Example Usage

```terraform
resource "ctyun_iam_group" "example-group" {
  name = "example-group"
}

data "ctyun_iam_group" "example-group" {
  group_name = ctyun_iam_group.example-group.name
}

output "group_arn" {
  value = data.ctyun_iam_group.example-group.arn
}
```

## Argument Reference

This data source supports the following arguments:

* `group_name` - (Required) The name of the IAM group. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the IAM group.
* `group_id` - ID of the IAM group.
* `id` - ID of the IAM group.
* `users` - Information about IAM users in the group. See [`users`](#users) for details.

### `users`

* `arn` - Resource name of the IAM user.
* `create_date` - Creation time of the IAM user.
* `password_last_used` - The last time that the user used the user password to log in to the OOS website.
* `user_id` - ID of the IAM user.
* `user_name` - Name of the IAM user.
