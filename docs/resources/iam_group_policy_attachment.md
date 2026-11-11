---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_group_policy_attachment"
description: |-
  Attaches a managed IAM policy to an IAM group.
---

# Resource: ctyun_iam_group_policy_attachment

Attaches an IAM policy to an IAM group.

## Example Usage

```terraform
resource "ctyun_iam_group" "example-group" {
  name = "example-group"
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

resource "ctyun_iam_group_policy_attachment" "example-attachment" {
  group      = ctyun_iam_group.example-group.name
  policy_arn = ctyun_iam_policy.example-policy.arn
}
```

## Argument Reference

This resource supports the following arguments:

* `group` - (Required, ForceNew) The name of the IAM group. The value is a string of 1 to 128 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `policy_arn` - (Required, ForceNew) The resource name of the policy. The value is a string of length 20 to 2048.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The attachment identifier in the form `groupname/policy_arn`.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM group policy attachment:

```terraform
import {
  to = ctyun_iam_group_policy_attachment.example-attachment
  identity = {
    group      = "example-group"
    policy_arn = "arn:ctyun:iam::1234567890123:policy/example-policy"
  }
}
```

### Identity Schema

#### Required

* `group` - The name of the IAM group.
* `policy_arn` - The resource name of the policy.

In Terraform v1.5.0 and later, use an `import` block with the group name and policy ARN separated by `/`:

```terraform
import {
  to = ctyun_iam_group_policy_attachment.example-attachment
  id = "example-group/arn:ctyun:iam::1234567890123:policy/example-policy"
}
```

Using `terraform import`, import an IAM group policy attachment using the group name and policy ARN separated by `/`:

```console
terraform import ctyun_iam_group_policy_attachment.example-attachment example-group/arn:ctyun:iam::1234567890123:policy/example-policy
```
