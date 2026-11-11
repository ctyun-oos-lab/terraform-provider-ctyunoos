---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_object_lock_configuration"
description: |-
  Provides an OOS bucket object lock configuration resource.
---

# Resource: ctyun_oos_bucket_object_lock_configuration

Provides an OOS bucket object lock configuration resource.

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

output "bucket_object_lock_configuration" {
  value = ctyun_oos_bucket_object_lock_configuration.example-object-lock
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket.
* `object_lock_enabled` - (Required) Whether the bucket enables the object lock function. Value: `Enabled` (Enable the object lock function), `Disabled` (Disable the object lock function). Note: once the object lock function is enabled, it cannot be disabled, and the object lock period cannot be shortened, but the object lock period can be extended.
* `rule` - (Optional) Object lock rule, supports `default_retention`. See [`rule`](#rule) for details.

### `rule`

* `default_retention` - (Required) Specific compliance retention rule configuration, supports `mode`, `days`, and `years`.

The `default_retention` configuration block supports the following argument:

* `mode` - (Required) Object lock mode. Value: `COMPLIANCE`.
* `days` - (Optional) Object lock days. Value: the value is an integer ranging from 1 to 36500. Note: the conversion relationship between years and days is: 1 year equals 365 days. Either `days` or `years` is required, but they cannot be specified together.
* `years` - (Optional) Object lock years. Value: the value is an integer ranging from 1 to 100. Note: the conversion relationship between years and days is: 1 year equals 365 days. Either `days` or `years` is required, but they cannot be specified together.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.

## Import

In Terraform v1.5.0 and later, use an `import` block to import an object lock configuration by the bucket name:

```terraform
import {
  to = ctyun_oos_bucket_object_lock_configuration.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the object lock configuration:

```console
terraform import ctyun_oos_bucket_object_lock_configuration.example example-oos-bucket
```
