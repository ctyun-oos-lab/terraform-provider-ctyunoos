---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user_group_membership"
description: |-
  Fetch information about IAM groups that the specified IAM user belongs to.
---

# Data Source: ctyun_iam_user_group_membership

This data source can be used to fetch information about IAM groups that the specified IAM user belongs to.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

resource "ctyun_iam_group" "example-group" {
  name = "example-group"
}

resource "ctyun_iam_user_group_membership" "example-membership" {
  user  = ctyun_iam_user.example-user.name
  group = ctyun_iam_group.example-group.name
}

data "ctyun_iam_user_group_membership" "example-membership" {
  user_name  = ctyun_iam_user.example-user.name
  depends_on = [ctyun_iam_user_group_membership.example-membership]
}

output "group_name_0" {
  value = data.ctyun_iam_user_group_membership.example-membership.groups_for_user.0.group_name
}
```

## Argument Reference

This data source supports the following arguments:

* `user_name` - (Required) The name of the IAM user. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `groups_for_user` - Information about IAM groups to which the specified IAM user belongs. See [`groups_for_user`](#groups_for_user) for details.
* `id` - Name of the IAM user.

### `groups_for_user`

* `arn` - Resource name of the IAM group.
* `create_date` - Creation time of the IAM group.
* `group_id` - ID of the IAM group.
* `group_name` - Name of the IAM group.
