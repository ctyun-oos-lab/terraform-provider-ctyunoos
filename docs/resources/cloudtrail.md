---
subcategory: "CloudTrail"
page_title: "CTYUN OOS: ctyun_cloudtrail"
description: |-
  Provides an OOS CloudTrail trail.
---

# Resource: ctyun_cloudtrail

Provides a cloudtrail resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_cloudtrail" "example-cloudtrail" {
  name           = "example-cloudtrail"
  s3_bucket_name = ctyun_oos_bucket.example-bucket.bucket
  s3_key_prefix  = "cloudtrail/"
  enable_logging = true

  event_selector {
    read_write_type = "All"
  }
}
```

## Argument Reference

This resource supports the following arguments:

* `name` - (Required, ForceNew) Specifies the name of the trail. Value: a string whose length is 3 to 128. The value can contain ASCII letters (`a-z`, `A-Z`), numbers (`0-9`), periods (`.`), underscores (`_`), or hyphens (`-`). The value must start with a letter or number, and end with a letter or number. The value must start with a letter or number, and end with a letter or number. The value cannot be an IP address (such as `192.162.5.4`). The value cannot contain any combination of adjacent period (.), underscores (`_`), and hyphens (`-`). Such as the value cannot contain similar combinations of dots (`..`), dots and underscores (`._`).
* `s3_bucket_name` - (Required) The name of OOS bucket where the trail log data is saved.
* `s3_key_prefix` - (Optional) Specifies the name prefix of the trail log. The value is a string whose length is 0 to 200.
* `enable_logging` - (Optional) Specifies whether to enable CloudTrail. Value: `true` (Enable CloudTrail), `false` (Disable CloudTrail). If this parameter is not specified, the server defaults to `true`.
* `event_selector` - (Optional) Specifies the management event selector, supports `read_write_type`. See [`event_selector`](#event_selector) for details.

### `event_selector`

* `read_write_type` - (Optional) The events to capture. Valid values are `All` (Read event and write event), `ReadOnly` (Read event), and `WriteOnly` (Write event). If this parameter is not specified, the server defaults to `All`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Resource name of the trail.
* `arn` - Resource name of the trail.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import a trail:

```terraform
import {
  to = ctyun_cloudtrail.example-cloudtrail
  identity = {
    arn = "arn:ctyun:cloudtrail::1234567890123:trail/example-cloudtrail"
  }
}
```

### Identity Schema

#### Required

* `arn` - Resource name of the trail.

In Terraform v1.5.0 and later, use an `import` block with the trail ARN:

```terraform
import {
  to = ctyun_cloudtrail.example-cloudtrail
  id = "arn:ctyun:cloudtrail::1234567890123:trail/example-cloudtrail"
}
```

It is also possible to import using the Terraform CLI by specifying the trail ARN:

```console
terraform import ctyun_cloudtrail.example-cloudtrail arn:ctyun:cloudtrail::1234567890123:trail/example-cloudtrail
```
