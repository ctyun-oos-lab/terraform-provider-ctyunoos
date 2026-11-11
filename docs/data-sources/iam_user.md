---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user"
description: |-
  Fetch information about the specified IAM user.
---

# Data Source: ctyun_iam_user

This data source can be used to fetch information about the specified IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

data "ctyun_iam_user" "example-user" {
  user_name = ctyun_iam_user.example-user.name
}

output "user_arn" {
  value = data.ctyun_iam_user.example-user.arn
}
```

## Argument Reference

This data source supports the following arguments:

* `user_name` - (Required) The name of the IAM user. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the IAM user.
* `id` - ID of the IAM user.
* `ip_last_used` - IP of the last login to the console.
* `password_last_used` - The last time that the user used the user password to log in to the OOS website.
* `tags` - Tags of the IAM user.
* `user_id` - ID of the IAM user.
