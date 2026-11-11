---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_server_side_encryption_configuration"
description: |-
  Fetch the server-side encryption configuration information of the bucket.
---

# Data Source: ctyun_oos_bucket_server_side_encryption_configuration

This data source can be used to fetch the server-side encryption configuration information of the bucket.

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

data "ctyun_oos_bucket_server_side_encryption_configuration" "example-server-side-encryption-data" {
  bucket = ctyun_oos_bucket_server_side_encryption_configuration.example-server-side-encryption.bucket
}

output "ctyun_oos_bucket_server_side_encryption_configuration_all_parameters" {
  value = data.ctyun_oos_bucket_server_side_encryption_configuration.example-server-side-encryption-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `rule` - Server-side encryption rule. See [`rule`](#rule) for details.

### `rule`

* `apply_server_side_encryption_by_default` - Server-side default encryption method.

The `apply_server_side_encryption_by_default` configuration block supports the following argument:

* `sse_algorithm` - The default encryption algorithm on the server side.
