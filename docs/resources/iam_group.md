---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_group"
description: |-
  Manages an IAM group.
---

# Resource: ctyun_iam_group

Provides an IAM resource to manage the IAM group.

## Example Usage

```terraform
resource "ctyun_iam_group" "example-group" {
  name = "example-group"
}
```

## Argument Reference

This resource supports the following arguments:

* `name` - (Required, ForceNew) The IAM group name. The value is a string of 1 to 128 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the IAM group.
* `arn` - Resource name of the IAM group.
* `unique_id` - ID of the IAM group.

## Import

In Terraform v1.5.0 and later, use an `import` block to import an IAM group by name:

```terraform
import {
  to = ctyun_iam_group.example-group
  id = "example-group"
}
```

Using `terraform import`, import an IAM group by name:

```console
terraform import ctyun_iam_group.example-group example-group
```
