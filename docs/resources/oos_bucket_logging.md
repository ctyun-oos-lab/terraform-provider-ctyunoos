---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_logging"
description: |-
  Provides an OOS bucket logging configuration resource.
---

# Resource: ctyun_oos_bucket_logging

Provides an OOS bucket logging configuration resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket" "example-target-bucket" {
  bucket = "example-target-bucket"
}

resource "ctyun_oos_bucket_logging" "example-logging" {
  bucket        = ctyun_oos_bucket.example-bucket.bucket
  target_bucket = ctyun_oos_bucket.example-target-bucket.bucket
  target_prefix = "log/"
}

output "bucket_logging" {
  value = ctyun_oos_bucket_logging.example-logging
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket.
* `target_bucket` - (Required) The bucket in which to store the server access logs.
* `target_prefix` - (Optional) The prefix for the keys that the log files will be stored under.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import a logging configuration:

```terraform
import {
  to = ctyun_oos_bucket_logging.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.

In Terraform v1.5.0 and later, use an `import` block to import a logging configuration by the source bucket name:

```terraform
import {
  to = ctyun_oos_bucket_logging.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the logging configuration:

```console
terraform import ctyun_oos_bucket_logging.example example-oos-bucket
```
