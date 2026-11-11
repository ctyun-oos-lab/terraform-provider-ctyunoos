---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user_group_membership"
description: |-
  Manages one IAM user membership in one IAM group.
---

# Resource: ctyun_iam_user_group_membership

Provides an IAM resource for adding an IAM user to an IAM group.

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
```

## Argument Reference

This resource supports the following arguments:

* `user` - (Required, ForceNew) The name of the IAM user.The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `group` - (Required, ForceNew) The name of the IAM group. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The membership identifier in the form `<user>/<group>`.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import a membership:

```terraform
import {
  to = ctyun_iam_user_group_membership.example-membership
  identity = {
    user  = "example-user"
    group = "example-group"
  }
}
```

### Identity Schema

#### Required

* `user` - IAM user name.
* `group` - IAM group name.

In Terraform v1.5.0 and later, use an `import` block with the user and group names separated by `/`:

```terraform
import {
  to = ctyun_iam_user_group_membership.example-membership
  id = "example-user/example-group"
}
```

Using `terraform import`, import a membership with the user and group names separated by `/`:

```console
terraform import ctyun_iam_user_group_membership.example-membership example-user/example-group
```
