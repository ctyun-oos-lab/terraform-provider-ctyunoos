---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_policy"
description: |-
  Provides an independent configuration resource for bucket policy configuration.
---

# Resource: ctyun_oos_bucket_policy

Provides an independent configuration resource for bucket policy configuration.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_policy" "example-policy" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "example-bucket-policy"
    Statement = [
      {
        Sid       = "AllowListBucket"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/example-user" }
        Action    = ["oos:ListBucket"]
        Resource  = "arn:ctyun:oos:::example-bucket"
      },
      {
        Sid       = "AllowReadObjects"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/example-user" }
        Action    = ["oos:GetObject"]
        Resource  = "arn:ctyun:oos:::example-bucket/*"
      }
    ]
  })
}

output "bucket_policy" {
  value = ctyun_oos_bucket_policy.example-policy
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket.
* `policy` - (Required) Text of a bucket policy. For details about the bucket policy, see "Object Oriented Storage API Reference".

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import a bucket policy:

```terraform
import {
  to = ctyun_oos_bucket_policy.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.

In Terraform v1.5.0 and later, use an `import` block to import a bucket policy by the bucket name:

```terraform
import {
  to = ctyun_oos_bucket_policy.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the bucket policy:

```console
terraform import ctyun_oos_bucket_policy.example example-oos-bucket
```
