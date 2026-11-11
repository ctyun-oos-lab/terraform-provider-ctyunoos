---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_policy"
description: |-
  Fetch the policy of the specified bucket.
---

# Data Source: ctyun_oos_bucket_policy

This data source can be used to fetch the policy of the specified bucket.

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
        Principal = {
          CTYUN = "arn:ctyun:iam:::user/example-user"
        }
        Action   = ["oos:ListBucket"]
        Resource = "arn:ctyun:oos:::example-bucket"
      },
      {
        Sid       = "AllowReadObjects"
        Effect    = "Allow"
        Principal = {
          CTYUN = "arn:ctyun:iam:::user/example-user"
        }
        Action   = ["oos:GetObject"]
        Resource = "arn:ctyun:oos:::example-bucket/*"
      }
    ]
  })
}

data "ctyun_oos_bucket_policy" "example-policy-data" {
  bucket = ctyun_oos_bucket_policy.example-policy.bucket
}

output "ctyun_oos_bucket_policy_all_parameters" {
  value = data.ctyun_oos_bucket_policy.example-policy-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `policy` - Text of the bucket policy.
