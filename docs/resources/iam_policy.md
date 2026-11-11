---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_policy"
description: |-
  Provides an IAM policy.
---

# Resource: ctyun_iam_policy

Provides an IAM policy resource.

## Example Usage

```terraform
resource "ctyun_iam_policy" "example-policy" {
  name        = "example-policy"
  description = "My example policy"

  # Terraform's "jsonencode" function converts a
  # Terraform expression result to valid JSON syntax.
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
```

## Argument Reference

This resource supports the following arguments:

* `name` - (Optional, ForceNew) The name of the policy. If neither `name` nor `name_prefix` is specified, Terraform will assign a random, unique name. Mutually exclusive with `name_prefix`. The value is a string of 1 to 128 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `description` - (Optional) The description of the policy. The value is a string of length 0 to 1,000.
* `name_prefix` - (Optional, ForceNew) The prefix of the policy name. If neither `name` nor `name_prefix` is specified, Terraform will assign a random, unique name. Mutually exclusive with `name`.
* `policy` - (Required) The JSON policy document. 

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the policy.
* `attachment_count` - The number of IAM roles, users, and groups associated with the policy.
* `id` - Resource name of the policy.
* `policy_id` - ID of the policy.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM policy:

```terraform
import {
  to = ctyun_iam_policy.example-policy
  identity = {
    arn = "arn:ctyun:iam::1234567890123:policy/example-policy"
  }
}
```

### Identity Schema

#### Required

* `arn` - Resource name of the policy.

In Terraform v1.5.0 and later, use an `import` block with the policy ARN as the ID:

```terraform
import {
  to = ctyun_iam_policy.example-policy
  id = "arn:ctyun:iam::1234567890123:policy/example-policy"
}
```

Using `terraform import`, import an IAM policy using the policy ARN:

```console
terraform import ctyun_iam_policy.example-policy arn:ctyun:iam::1234567890123:policy/example-policy
```
