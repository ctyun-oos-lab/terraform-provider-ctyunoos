---
subcategory: "CloudTrail"
page_title: "CTYUN OOS: ctyun_cloudtrail"
description: |-
  Fetch the trail status information.
---

# Data Source: ctyun_cloudtrail

This data source can be used to fetch the trail status information.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_cloudtrail" "example-cloudtrail" {
  name           = "example-cloudtrail"
  s3_bucket_name = ctyun_oos_bucket.example-bucket.bucket
}

data "ctyun_cloudtrail" "example-cloudtrail" {
  name = ctyun_cloudtrail.example-cloudtrail.name
}

output "example_cloudtrail_all_parameters" {
  value = data.ctyun_cloudtrail.example-cloudtrail
}
```

## Argument Reference

This data source supports the following argument:

* `name` - (Required) Specifies the name of the trail.

## Attribute Reference

This data source exports the following attributes in addition to the argument above:

* `id` - Resource name of the trail.
* `arn` - Resource name of the trail.
* `enable_logging` - Whether to enable CloudTrail. Value: `true` (Enable CloudTrail), `false` (Disable CloudTrail).
* `event_selector` - The management event selector. See [`event_selector`](#event_selector) for details.
* `latest_delivery_error` - The error was encountered when saving logs to Bucket the last time.
* `latest_delivery_time` - Time when the trail log was last saved to Bucket.
* `s3_bucket_name` - The name of OOS bucket where the trail log data is saved.
* `s3_key_prefix` - Name prefix of the trail log.
* `start_logging_time` - The last start time of the trail.
* `stop_logging_time` - The last stop time of the trail.

### `event_selector`

* `read_write_type` - The write-read type of the management event. Value: `ReadOnly` (Read event), `WriteOnly` (Write event), `All` (Read event and write event).
