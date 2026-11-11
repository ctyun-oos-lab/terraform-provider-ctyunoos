---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_role_policy_attachment"
description: |-
  Attaches a managed IAM policy to an IAM role.
---

# Resource: ctyun_iam_role_policy_attachment

Attaches an IAM policy to an IAM role.

## Example Usage

```terraform
resource "ctyun_iam_role" "example-role" {
  name = "example-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "iam:AssumeRole"
        Effect = "Allow"
        Principal = {
          CTYUN = "arn:ctyun:iam::1234567890123:user/example-user"
        }
      },
    ]
  })
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

resource "ctyun_iam_role_policy_attachment" "example-attachment" {
  role       = ctyun_iam_role.example-role.name
  policy_arn = ctyun_iam_policy.example-policy.arn
}
```

## Argument Reference

This resource supports the following arguments:

* `role` - (Required, ForceNew) The name of the IAM role. The name of the IAM role must be unique within the OOS account. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `policy_arn` - (Required, ForceNew) The resource name of the policy. The value is a string of length 20 to 2048.

Do not configure arguments that are not listed above.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - The attachment identifier in the form `rolename/policy_arn`.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM role policy attachment:

```terraform
import {
  to = ctyun_iam_role_policy_attachment.example-attachment
  identity = {
    role       = "example-role"
    policy_arn = "arn:ctyun:iam::1234567890123:policy/example-policy"
  }
}
```

### Identity Schema

#### Required

* `role` - The name of the IAM role.
* `policy_arn` - The resource name of the policy.

In Terraform v1.5.0 and later, use an `import` block with the role name and policy ARN separated by `/`:

```terraform
import {
  to = ctyun_iam_role_policy_attachment.example-attachment
  id = "example-role/arn:ctyun:iam::1234567890123:policy/example-policy"
}
```

Using `terraform import`, import an IAM role policy attachment using the role name and policy ARN separated by `/`:

```console
terraform import ctyun_iam_role_policy_attachment.example-attachment example-role/arn:ctyun:iam::1234567890123:policy/example-policy
```
