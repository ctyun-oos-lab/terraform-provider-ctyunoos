---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_role_policy_attachments"
description: |-
  Fetch the managed policies attached to a specific IAM role.
---

# Data Source: ctyun_iam_role_policy_attachments

This data source can be used to fetch the managed policies attached to a specific IAM role.

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
      }
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

data "ctyun_iam_role_policy_attachments" "example-attachments" {
  role_name  = ctyun_iam_role.example-role.name
  depends_on = [ctyun_iam_role_policy_attachment.example-attachment]
}

output "attached_policy_arn_0" {
  value = data.ctyun_iam_role_policy_attachments.example-attachments.attached_policies.0.policy_arn
}
```

## Argument Reference

This data source supports the following arguments:

* `role_name` - (Required) The name of the IAM role. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `attached_policies` - Information about policies attached to the specified IAM role. See [`attached_policies`](#attached_policies) for details.
* `id` - Name of the IAM role.

### `attached_policies`

* `description` - Description of the policy.
* `policy_arn` - Resource name of the policy.
* `policy_name` - Name of the policy.
* `scope` - The type of the policy. Value: `Local` (User-defined policy), `OOS` (System-defined policy).
