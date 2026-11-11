---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user_policy_attachments"
description: |-
  Fetch information about policies attached to the specified IAM user.
---

# Data Source: ctyun_iam_user_policy_attachments

This data source can be used to fetch information about policies attached to the specified IAM user.

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

data "ctyun_iam_user_policy_attachments" "example-attachments" {
  user_name  = ctyun_iam_user.example-user.name
  depends_on = [ctyun_iam_user_policy_attachment.example-attachment]
}

output "attached_policy_arn_0" {
  value = data.ctyun_iam_user_policy_attachments.example-attachments.attached_policies.0.policy_arn
}
```

## Argument Reference

This data source supports the following arguments:

* `user_name` - (Required) The name of the IAM user. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `attached_policies` - Information about policies attached to the specified IAM user. See [`attached_policies`](#attached_policies) for details.
* `id` - Name of the IAM user.

### `attached_policies`

* `description` - Description of the policy.
* `policy_arn` - Resource name of the policy.
* `policy_name` - Name of the policy.
* `scope` - The type of the policy. Value: `Local` (User-defined policy), `OOS` (System-defined policy).
