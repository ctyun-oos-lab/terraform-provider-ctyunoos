---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_policies"
description: |-
  Fetch information about specified policies or all policies.
---

# Data Source: ctyun_iam_policies

This data source can be used to fetch information about specified policies or all policies.

## Example Usage

### Policies filtered by name regex

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

data "ctyun_iam_policies" "example-policies" {
  name_regex = ctyun_iam_policy.example-policy.name
}

output "policies_name_0" {
  value = data.ctyun_iam_policies.example-policies.policies.0.name
}
```

## Argument Reference

This data source supports the following arguments:

* `name_regex` - (Optional) A regular expression used to filter by name.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arns` - Resource names of matched policies.
* `id` - Region name.
* `names` - Names of matched policies.
* `policies` - Information about policies. See [`policies`](#policies) for details.

### `policies`

* `arn` - Resource name of the policy.
* `attachment_count` - The number of IAM roles, users, and groups associated with the policy.
* `create_date` - Creation time of the policy.
* `description` - Description of the policy.
* `name` - Name of the policy.
* `policy_id` - ID of the policy.
* `scope` - The type of the policy. Value: `Local` (User-defined policy), `OOS` (System-defined policy).
