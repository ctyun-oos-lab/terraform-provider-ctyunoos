---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_user"
description: |-
  Provides an IAM user.
---

# Resource: ctyun_iam_user

Provides an IAM resource to manage the IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name          = "example-user"
  force_destroy = true

  tags = {
    managed_by  = "terraform"
    environment = "production"
  }
}
```

## Argument Reference

This resource supports the following arguments:

* `name` - (Required, ForceNew) The name of the IAM user. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `force_destroy` - (Optional, default `false`) Specifies whether to remove resources associated with the IAM user, including console login password, access keys, group memberships, MFA binding, MFA devices, and policy attachments. When delete this user, IAM user must meet the following conditions: no login password for the console, no access key, no user group joining, no MFA binding, no MFA device, and no policy attachment. Value: `true` (Remove resources associated with the IAM user), `false` (Does not remove resources associated with the IAM user). If this parameter is not specified, the server defaults to `false`.
* `tags` - (Optional) Tags of the IAM user. The format is `key=value`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the IAM user.
* `arn` - Resource name of the IAM user.
* `unique_id` - ID of the IAM user.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing IAM user:

```terraform
import {
  to = ctyun_iam_user.example-user
  identity = {
    name = "example-user"
  }
}
```

### Identity Schema

#### Required

* `name` - Name of the IAM user.

In Terraform v1.5.0 and later, use an `import` block with the user name as the ID:

```terraform
import {
  to = ctyun_iam_user.example-user
  id = "example-user"
}
```

Using `terraform import`, import an IAM user using the user name:

```console
terraform import ctyun_iam_user.example-user example-user
```

The service does not return the `force_destroy` configuration during import.
Set `force_destroy` explicitly after import when the user has dependencies that are not managed by Terraform, and review the resulting plan before applying it.

## Lifecycle Notes

* The user name is an immutable identifier. Changing `name` destroys the existing user and creates a new one. Migrate dependent resources and access permissions before making this change in a production environment.
* When `force_destroy` is `false`, destroying a user with undeleted dependencies may fail. Delete the dependencies first, or enable `force_destroy` after confirming the cleanup scope.
* If the user is deleted outside Terraform, refresh removes it from state and the next plan proposes creating it again.
* Tags are managed by Terraform. If tags are changed outside Terraform, the next plan detects the drift and `apply` restores the configured values.
