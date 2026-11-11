---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_object_lock_configuration"
description: |-
  Fetch the configuration information of object lock.
---

# Data Source: ctyun_oos_bucket_object_lock_configuration

This data source can be used to fetch the configuration information of object lock.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_object_lock_configuration" "example-object-lock" {
  bucket              = ctyun_oos_bucket.example-bucket.bucket
  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 2
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "example-object-lock-data" {
  bucket = ctyun_oos_bucket_object_lock_configuration.example-object-lock.bucket
}

output "ctyun_oos_bucket_object_lock_configuration_all_parameters" {
  value = data.ctyun_oos_bucket_object_lock_configuration.example-object-lock-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `object_lock_enabled` - Whether the bucket enables the object lock function. Value: `Enabled` (The object lock function is enabled), `Disabled` (the object lock function is not enabled).
* `rule` - Object lock rule. See [`rule`](#rule) for details.

### `rule`

* `default_retention` - Specific compliance retention rule configuration.

The `default_retention` configuration block supports the following argument:

* `mode` - Object lock mode.
* `days` - Object lock days.
* `years` - Object lock years.
