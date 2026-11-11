---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_soft_delete_configuration"
description: |-
  Fetch the soft delete configuration information of the bucket.
---

# Data Source: ctyun_oos_bucket_soft_delete_configuration

This data source can be used to fetch the soft delete configuration information of the bucket.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_soft_delete_configuration" "example-soft-delete" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  days   = 13
}

data "ctyun_oos_bucket_soft_delete_configuration" "example-soft-delete-data" {
  bucket = ctyun_oos_bucket_soft_delete_configuration.example-soft-delete.bucket
}

output "ctyun_oos_bucket_soft_delete_configuration_all_parameters" {
  value = data.ctyun_oos_bucket_soft_delete_configuration.example-soft-delete-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `days` - Retention period for soft delete.
