---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_groups"
description: |-
  Fetch information about a set of IAM groups.
---

# Data Source: ctyun_iam_groups

This data source can be used to fetch information about specified IAM groups or all IAM groups.

## Example Usage

```terraform
resource "ctyun_iam_group" "example-group" {
  name = "example-group"
}

data "ctyun_iam_groups" "example-groups" {
  name_regex = ctyun_iam_group.example-group.name
}

output "groups_name_0" {
  value = data.ctyun_iam_groups.example-groups.groups.0.group_name
}
```

## Argument Reference

This data source supports the following arguments:

* `name_regex` - (Optional) A regular expression used to filter by name.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `arns` - Resource names of matched IAM groups.
* `id` - Region name.
* `names` - Names of matched groups.
* `groups` - Information about IAM groups. See [`groups`](#groups) for details.

### `groups`

* `arn` - Resource name of the IAM group.
* `create_date` - Creation time of the IAM group.
* `group_id` - ID of the IAM group.
* `group_name` - Name of the IAM group.
* `policies` - The number of policies associated with the IAM group.
* `users` - The number of users in the IAM group.
