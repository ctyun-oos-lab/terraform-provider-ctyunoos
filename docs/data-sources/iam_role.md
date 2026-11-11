---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_role"
description: |-
  Fetch information about a specific IAM role.
---

# Data Source: ctyun_iam_role

This data source can be used to fetch information about a specific IAM role.

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

data "ctyun_iam_role" "example-role" {
  name = ctyun_iam_role.example-role.name
}

output "role_arn" {
  value = data.ctyun_iam_role.example-role.arn
}
```

## Argument Reference

This data source supports the following arguments:

* `name` - (Required) The name of the IAM role. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the IAM role.
* `arn` - Resource name of the IAM role.
* `assume_role_policy` - Trust policy.
* `create_date` - Creation time of the IAM role.
* `description` - The description of the IAM role.
* `max_session_duration` - The maximum duration (in seconds) of a session established by assuming the role.
* `unique_id` - ID of the IAM role.
