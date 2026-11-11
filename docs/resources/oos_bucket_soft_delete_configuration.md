---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_soft_delete_configuration"
description: |-
  Provides an OOS bucket soft delete configuration resource.
---

# Resource: ctyun_oos_bucket_soft_delete_configuration

Provides an OOS bucket soft delete configuration resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_soft_delete_configuration" "example-soft-delete" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  days   = 13
}

output "bucket_soft_delete_configuration" {
  value = ctyun_oos_bucket_soft_delete_configuration.example-soft-delete
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket.
* `days` - (Required) Specifies retention period for soft delete. The value is an integer ranging from 1 to 15. When the retention period is shortened, the objects in the soft delete list will be updated, and the expired objects will be queued for deletion immediately. When the retention period is extended, the objects in the soft delete list will be updated, and the expiration deletion policy will be executed according to the latest retention period.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import a soft delete configuration:

```terraform
import {
  to = ctyun_oos_bucket_soft_delete_configuration.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.

In Terraform v1.5.0 and later, use an `import` block to import a soft delete configuration by the bucket name:

```terraform
import {
  to = ctyun_oos_bucket_soft_delete_configuration.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the soft delete configuration:

```console
terraform import ctyun_oos_bucket_soft_delete_configuration.example example-oos-bucket
```
