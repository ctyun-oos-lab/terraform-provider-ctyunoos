---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user_policy_attachment"
description: |-
  Attaches a managed IAM policy to an IAM user.
---

# Resource: ctyun_iam_user_policy_attachment

Attaches an IAM policy to an IAM user.

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
```

## Argument Reference

This resource supports the following arguments:

* `user` - (Required, ForceNew) The name of the IAM user. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `policy_arn` - (Required, ForceNew) The resource name of the policy. The value is a string of length 20 to 2048.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The attachment identifier in the form `username/policy_arn`.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM user policy attachment:

```terraform
import {
  to = ctyun_iam_user_policy_attachment.example-attachment
  identity = {
    user       = "example-user"
    policy_arn = "arn:ctyun:iam::1234567890123:policy/example-policy"
  }
}
```

### Identity Schema

#### Required

* `user` - The name of the IAM user.
* `policy_arn` - The resource name of the policy.

In Terraform v1.5.0 and later, use an `import` block with the user name and policy ARN separated by `/`:

```terraform
import {
  to = ctyun_iam_user_policy_attachment.example-attachment
  id = "example-user/arn:ctyun:iam::1234567890123:policy/example-policy"
}
```

Using `terraform import`, import an IAM user policy attachment using the user name and policy ARN separated by `/`:

```console
terraform import ctyun_iam_user_policy_attachment.example-attachment example-user/arn:ctyun:iam::1234567890123:policy/example-policy
```
