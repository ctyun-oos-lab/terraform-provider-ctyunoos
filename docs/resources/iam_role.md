---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_role"
description: |-
  Provides an IAM role.
---

# Resource: ctyun_iam_role

Provides an IAM role.

## Example Usage

```terraform
resource "ctyun_iam_role" "example-role" {
  name                 = "example-role"
  description          = "My example role"
  max_session_duration = 7200

  # Terraform's "jsonencode" function converts a
  # Terraform expression result to valid JSON syntax.
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
```

## Argument Reference

This resource supports the following arguments:

* `name` - (Optional, ForceNew) The name of the IAM role. If neither `name` nor `name_prefix` is specified, Terraform will assign a random, unique name. Mutually exclusive with `name_prefix`. The value is a string of 1 to 64 letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `name_prefix` - (Optional, ForceNew) The prefix of the role name. If neither `name` nor `name_prefix` is specified, Terraform will assign a random, unique name. Mutually exclusive with `name`.
* `assume_role_policy` - (Required) The JSON trust policy document. 
* `description` - (Optional) The description of the IAM role. The value is a string of length 0 to 1,000.
* `max_session_duration` - (Optional) Specifies the maximum duration of a session established by assuming the role. The value is an integer that ranges from 3600 to 43200. The unit is second. If this parameter is not specified, the server defaults to 3600.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the IAM role.
* `create_date` - Creation time of the IAM role.
* `id` - Name of the IAM role.
* `unique_id` - ID of the IAM role.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM role:

```terraform
import {
  to = ctyun_iam_role.example-role
  identity = {
    name = "example-role"
  }
}
```

### Identity Schema

#### Required

* `name` - The name of the IAM role.

In Terraform v1.5.0 and later, use an `import` block with the role name as the ID:

```terraform
import {
  to = ctyun_iam_role.example-role
  id = "example-role"
}
```

Using `terraform import`, import an IAM role using the role name:

```console
terraform import ctyun_iam_role.example-role example-role
```
