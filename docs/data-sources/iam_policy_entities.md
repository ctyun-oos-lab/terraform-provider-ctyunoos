---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_policy_entities"
description: |-
  Fetch the users, groups, and roles that an IAM managed policy is attached to.
---

# Data Source: ctyun_iam_policy_entities

This data source can be used to fetch the users, groups, and roles that an IAM managed policy is attached to.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

resource "ctyun_iam_policy" "example-policy" {
  name        = "example-policy"
  description = "An example policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = ["oos:ListBucket"]
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "ctyun_iam_user_policy_attachment" "example-attachment" {
  user       = ctyun_iam_user.example-user.name
  policy_arn = ctyun_iam_policy.example-policy.arn
}

data "ctyun_iam_policy_entities" "example-entities" {
  arn        = ctyun_iam_policy.example-policy.arn
  depends_on = [ctyun_iam_user_policy_attachment.example-attachment]
}

output "policy_users" {
  value = data.ctyun_iam_policy_entities.example-entities.users
}
```

## Argument Reference

This data source supports the following arguments:

* `arn` - (Required) Resource name of the policy. The value is a string of length 20 to 2048.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Resource name of the policy.
* `groups` - A list of groups to whom the specified policy is attached. See [`groups`](#groups) for details.
* `roles` - A list of roles whom the specified policy is attached. See [`roles`](#roles) for details.
* `users` - A list of users to whom the specified policy is attached. See [`users`](#users) for details.

### `groups`

* `group_name` - Name of the IAM group.
* `group_id` - ID of the IAM group.

### `roles`

* `role_name` - Name of the IAM role.
* `role_id` - ID of the IAM role.

### `users`

* `user_name` - Name of the IAM user.
* `user_id` - ID of the IAM user.
