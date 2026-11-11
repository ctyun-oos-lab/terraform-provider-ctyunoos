---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_policy"
description: |-
  Fetch information about a specific IAM managed policy.
---

# Data Source: ctyun_iam_policy

This data source can be used to fetch information about a specific IAM managed policy.

## Example Usage

```terraform
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

data "ctyun_iam_policy" "example-policy" {
  name = ctyun_iam_policy.example-policy.name
}

output "policy_arn" {
  value = data.ctyun_iam_policy.example-policy.arn
}
```

## Argument Reference

This data source supports the following arguments:

* `name` - (Optional) The name of the policy. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`). `name` and `arn` cannot both be specified, exactly one of them must be provided.
* `arn` - (Optional) Resource name of the policy. The value is a string of length 20 to 2048. `name` and `arn` cannot both be specified, exactly one of them must be provided.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `attachment_count` - The number of IAM roles, users, and groups associated with the policy.
* `create_date` - Creation time of the policy.
* `description` - Description of the policy.
* `id` - Resource name of the policy.
* `policy` - The document of the policy.
* `policy_id` - ID of the policy.
* `scope` - The type of the policy. Value: `Local` (User-defined policy), `OOS` (System-defined policy).
