---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_group_policy_attachments"
description: |-
  Fetch information about managed policies attached to an IAM group.
---

# Data Source: ctyun_iam_group_policy_attachments

This data source can be used to fetch information about managed policies attached to a specific IAM group.

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

data "ctyun_iam_group_policy_attachments" "example-attachments" {
  group_name = ctyun_iam_group.example-group.name
  depends_on = [ctyun_iam_group_policy_attachment.example-attachment]
}

output "attached_policy_arn_0" {
  value = data.ctyun_iam_group_policy_attachments.example-attachments.attached_policies.0.policy_arn
}
```

## Argument Reference

This data source supports the following arguments:

* `group_name` - (Required) The name of the IAM group. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `attached_policies` - Information about policies attached to the specified IAM group. See [`attached_policies`](#attached_policies) for details.
* `id` - Name of the IAM group.

### `attached_policies`

* `description` - Description of the policy.
* `policy_arn` - Resource name of the policy.
* `policy_name` - Name of the policy.
* `scope` - The type of the policy. Value: `Local` (User-defined policy), `OOS` (System-defined policy).
