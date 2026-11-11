---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_server_side_encryption_configuration"
description: |-
  Provides an OOS bucket server-side encryption configuration resource.
---

# Resource: ctyun_oos_bucket_server_side_encryption_configuration

Provides an OOS bucket server-side encryption configuration resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "example-server-side-encryption" {
  bucket = ctyun_oos_bucket.example-bucket.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

output "bucket_server_side_encryption_configuration" {
  value = ctyun_oos_bucket_server_side_encryption_configuration.example-server-side-encryption
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket.
* `rule` - (Required) Server-side encryption rule, supports `apply_server_side_encryption_by_default`. See [`rule`](#rule) for details.

### `rule`

* `apply_server_side_encryption_by_default` - (Required) Server-side default encryption method, supports `sse_algorithm`.

The `apply_server_side_encryption_by_default` configuration block supports the following argument:

* `sse_algorithm` - (Required) The default encryption algorithm on the server side. Value: the value is case-sensitive and can only be set to `AES256`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import an encryption configuration:

```terraform
import {
  to = ctyun_oos_bucket_server_side_encryption_configuration.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.

In Terraform v1.5.0 and later, use an `import` block to import an encryption configuration by the bucket name:

```terraform
import {
  to = ctyun_oos_bucket_server_side_encryption_configuration.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the encryption configuration:

```console
terraform import ctyun_oos_bucket_server_side_encryption_configuration.example example-oos-bucket
```
