---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket"
description: |-
  Provides an OOS bucket resource.
---

# Resource: ctyun_oos_bucket

Provides an OOS bucket resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-access-log" {
  bucket = "example-bucket-access-log"
}

resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
  acl    = "public-read"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicObjectRead"
        Effect    = "Allow"
        Principal = "*"
        Action    = ["oos:GetObject"]
        Resource  = "arn:ctyun:oos:::example-bucket/*"
      }
    ]
  })
  logging {
    target_bucket = ctyun_oos_bucket.example-access-log.bucket
    target_prefix = "access-logs/"
  }
  lifecycle_rule {
    id      = "expire-objects"
    enabled = true
    prefix  = "logs/"
    expiration {
      days = 365
    }
  }
  lifecycle_rule {
    id      = "date-expire-objects"
    enabled = false
    prefix  = "temp/"
    expiration {
      date = "2027-06-30T00:00:00.000Z"
    }
  }
  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
  object_lock_configuration {
    object_lock_enabled = "Enabled"
    rule {
      default_retention {
        mode = "COMPLIANCE"
        days = 60
      }
    }
  }
}

output "bucket" {
  value = ctyun_oos_bucket.example-bucket
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Optional, ForceNew) The name for the bucket. Mutually exclusive with `bucket_prefix`. If neither `bucket` nor `bucket_prefix` is specified, a bucket name will be generated automatically in the form of the prefix `terraform-` followed by a 26-character random hexadecimal string. Value: A string whose length is 3 to 63, and the bucket name must be globally unique. It can only consist of lowercase letters, numbers, dashes (`-`), and dots (`.`). Bucket name can consist of one or more sub-sections, separated by dots (`.`). Each sub-section needs to start with lowercase letters or numbers, or end with lowercase letters or numbers. Bucket name cannot be in the form of IP addresses (such as `192.162.0.1`). Bucket name cannot be a combination of one or more sets of "number.number". Bucket name cannot contain double horizontal lines (`--`), double dots (`..`), horizontal dots (`-.`), and horizontal dots (`.-`). It is not allowed to use illegal and sensitive characters, such as information related to violent terrorist affairs.
* `bucket_prefix` - (Optional, ForceNew) The prefix of the bucket name. Mutually exclusive with `bucket`. If neither `bucket` nor `bucket_prefix` is specified, a bucket name will be generated automatically in the form of the prefix `terraform-` followed by a 26-character random hexadecimal string. Value: The value is a string of length 3 to 37.
* `force_destroy` - (Optional, default `false`) Specifies whether to allow deleting a non-empty bucket. The bucket can be deleted only when it contains no objects and no objects are in the soft delete retention period. Value: `true` (Allow deleting a non-empty bucket), `false` (Does not allow deleting a non-empty bucket). If this parameter is not specified, the server defaults to `false`.
* `acl` - (Optional, Deprecated) Sets the ACL for the bucket. Value:
  * `private` - Only the account and IAM users/roles with corresponding permissions can read/write/delete objects in the bucket (including Put, Delete, and Get Object). Other people (including anonymous) can only access objects in the bucket if they are authorized by the bucket policy or share a link.
  * `public-read` - Only the account and IAM users/roles with corresponding permissions can write/delete objects in the bucket (including Put and Delete Object). Anyone (including anonymous) can read objects in the bucket. This may result in unexpected access to the bucket and high costs. Please use this permission carefully!
  * `public-read-write` - Anyone (including anonymous) can read/write/delete objects in the bucket. This may result in unexpected access to the bucket and high costs. Your legitimate rights and interests may be infringed if someone uploads illegal information. Therefore, we recommend that you do not set the bucket ACL to `public-read-write` except in special cases.
  If this parameter is not specified, the server defaults to `private`. Use the resource `ctyun_oos_bucket_acl` instead.
* `lifecycle_rule` - (Optional, Deprecated) Bucket lifecycle configuration. When there are multiple rules, you can specify multiple `lifecycle_rule` blocks. Use the resource `ctyun_oos_bucket_lifecycle_configuration` instead. See [`lifecycle_rule`](#lifecycle_rule) for details.
* `logging` - (Optional, Deprecated) Bucket logging configuration. Use the resource `ctyun_oos_bucket_logging` instead. See [`logging`](#logging) for details.
* `object_lock_configuration` - (Optional, Deprecated) Object lock configuration. Use the resource `ctyun_oos_bucket_object_lock_configuration` instead. See [`object_lock_configuration`](#object_lock_configuration) for details.
* `policy` - (Optional, Deprecated) Bucket policy configuration. Text of a bucket policy. For details about the bucket policy, see Object Oriented Storage API Reference. Use the resource `ctyun_oos_bucket_policy` instead.
* `server_side_encryption_configuration` - (Optional, Deprecated) Server-side encryption configuration. Use the resource `ctyun_oos_bucket_server_side_encryption_configuration` instead. See [`server_side_encryption_configuration`](#server_side_encryption_configuration) for details.

### `lifecycle_rule`

* `id` - (Optional) Specifies unique identifier for the lifecycle rule. Value: 1~255 strings. When not specified, it is generated by the provider client: the prefix `tf-s3-lifecycle-` followed by a 26-character hexadecimal string, for a total of 42 characters.
* `prefix` - (Required) Specifies the object prefix of the lifecycle rule. Only objects that match the object prefix can be affected by this rule. In the same bucket, the lifecycle rules of the same type, cannot have superimposed prefixes. If the value is `null`, the lifecycle rule matches the entire bucket. Value: 0~1024 strings.
* `enabled` - (Required) Specifies the status of the lifecycle rule. Value: `true`: Lifecycle rule takes effect. `false`: OOS ignores the lifecycle rule.
* `expiration` - (Optional) The expiration time rule, supports `days`, `date`.

The `expiration` configuration block supports the following argument:

* `days` - (Optional) Specifies how many days after the creation of the matched object, the lifecycle rule takes effect. Value: Positive integer. Either `days` or `date` is required, but they cannot be specified together.
* `date` - (Optional) Specifies the effective date of the lifecycle rule. OOS executes the lifecycle rule on objects created before this date. At this effective time, objects whose generation time is earlier than this time will be considered expired objects. Value: The date must be in ISO 8601 format at midnight UTC, in the format `YYYY-MM-DDT00:00:00.000Z`, for example `2026-10-11T00:00:00.000Z`. Either `days` or `date` is required, but they cannot be specified together.

### `logging`

* `target_bucket` - (Required) The bucket in which to store the server access logs.
* `target_prefix` - (Optional) The prefix for the keys that the log files will be stored under.

### `object_lock_configuration`

* `object_lock_enabled` - (Required) Whether the bucket enables the object lock function. Value: `Enabled`: enable the object lock function. `Disabled`: disable the object lock function. Note: Once the object lock function is enabled, it cannot be disabled, and the object lock period cannot be shortened, but the object lock period can be extended.
* `rule` - (Optional) Object lock rule, supports `default_retention`.

The `rule` configuration block supports the following argument:

* `default_retention` - (Required) Specific compliance retention rule configuration, supports `mode`, `days` or `years`.

The `default_retention` configuration block supports the following argument:

* `mode` - (Required) Object lock mode. Value: `COMPLIANCE`.
* `days` - (Optional) Object lock days. Value: The value is an integer ranging from 1 to 36500. Note: the conversion relationship between years and days is: 1 year equals 365 days. Either `days` or `years` is required, but they cannot be specified together.
* `years` - (Optional) Object lock years. Value: The value is an integer ranging from 1 to 100. Note: the conversion relationship between years and days is: 1 year equals 365 days. Either `days` or `years` is required, but they cannot be specified together.

### `server_side_encryption_configuration`

* `rule` - (Required) Server-side encryption rule.

The `rule` configuration block supports the following argument:

* `apply_server_side_encryption_by_default` - (Required) Server-side default encryption method.

The `apply_server_side_encryption_by_default` configuration block supports the following argument:

* `sse_algorithm` - (Required) The default encryption algorithm on the server side. Value: The value is case-sensitive and can only be set to `AES256`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the bucket.
* `id` - Name of the Bucket.

## Import

In Terraform v1.12.0 and later, the `identity` attribute of an `import` block can be used to import an existing OOS bucket:

```terraform
import {
  to = ctyun_oos_bucket.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.

In Terraform v1.5.0 and later, use an `import` block with the bucket name as the ID:

```terraform
import {
  to = ctyun_oos_bucket.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import an OOS bucket using the bucket name:

```console
terraform import ctyun_oos_bucket.example example-oos-bucket
```

## Lifecycle Notes

* Import sets `force_destroy` to `false` on the imported resource state. Set `force_destroy` explicitly after import when the bucket contains objects that are not managed by Terraform, and review the resulting plan before applying it.
* The deprecated `acl`, `lifecycle_rule`, `logging`, `object_lock_configuration`, `policy`, and `server_side_encryption_configuration` arguments remain supported for backward compatibility but are no longer recommended for new configurations. Use the dedicated `ctyun_oos_bucket_acl`, `ctyun_oos_bucket_lifecycle_configuration`, `ctyun_oos_bucket_logging`, `ctyun_oos_bucket_object_lock_configuration`, `ctyun_oos_bucket_policy`, and `ctyun_oos_bucket_server_side_encryption_configuration` resources instead.
