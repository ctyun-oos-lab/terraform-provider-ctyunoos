---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_roles"
description: |-
  Fetch information about specified IAM roles or all IAM roles.
---

# Data Source: ctyun_iam_roles

This data source can be used to fetch information about specified IAM roles or all IAM roles.

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

data "ctyun_iam_roles" "example-roles" {
  name_regex = ctyun_iam_role.example-role.name
}

output "roles_name_0" {
  value = data.ctyun_iam_roles.example-roles.roles.0.name
}
```

## Argument Reference

This data source supports the following arguments:

* `name_regex` - (Optional) A regular expression used to filter by name.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arns` - Resource names of matched IAM roles.
* `id` - Region name.
* `names` - Names of matched roles.
* `roles` - Information about IAM roles. See [`roles`](#roles) for details.

### `roles`

* `arn` - Resource name of the IAM role.
* `assume_role_policy` - Trust policy.
* `create_date` - Creation time of the IAM role.
* `description` - The description of the IAM role.
* `max_session_duration` - The maximum duration (in seconds) of a session established by assuming the role.
* `name` - Name of the IAM role.
* `unique_id` - ID of the IAM role.
