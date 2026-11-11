---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_lifecycle_configuration"
description: |-
  Fetch the configured bucket lifecycle rules.
---

# Data Source: ctyun_oos_bucket_lifecycle_configuration

This data source can be used to fetch the configured bucket lifecycle rules.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_lifecycle_configuration" "example-lifecycle" {
  bucket = ctyun_oos_bucket.example-bucket.bucket

  rule {
    id     = "days-stable"
    prefix = "days/"
    status = "Enabled"

    expiration {
      days = 365
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "example-lifecycle-data" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.example-lifecycle.bucket
}

output "ctyun_oos_bucket_lifecycle_configuration_all_parameters" {
  value = data.ctyun_oos_bucket_lifecycle_configuration.example-lifecycle-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `rule` - The rule configuration for the bucket. When multiple rules exist, multiple `rule` blocks are returned. See [`rule`](#rule) for details.

### `rule`

* `id` - Unique identifier for the lifecycle rule.
* `prefix` - The object prefix of the lifecycle rule.
* `status` - The status of the lifecycle rule. `Enabled`: the lifecycle rule takes effect. `Disabled`: OOS ignores the lifecycle rule.
* `expiration` - The expiration time rule, supports `days` and `date`.

The `expiration` configuration block supports the following argument:

* `days` - The number of days after the creation of the matched object when the lifecycle rule takes effect.
* `date` - The effective date of the lifecycle rule.
