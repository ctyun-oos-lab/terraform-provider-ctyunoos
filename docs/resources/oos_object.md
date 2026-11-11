---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_object"
description: |-
  Provides an OOS object resource.
---

# Resource: ctyun_oos_object

Provides an OOS object resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_object" "example-object-source" {
  bucket             = ctyun_oos_bucket.example-bucket.bucket
  key                = "examples/object-with-source.txt"
  source             = "${path.module}/source-object.txt"
  cache_control      = "max-age=3600"
  content_disposition = "attachment; filename=object-with-source.txt"
  content_encoding   = "identity"
  content_md5        = "K9Ch40Q8Pg6VnoRC3hBL4g=="
  content_type       = "text/plain"
  etag               = filemd5("${path.module}/source-object.txt")
  forbid_overwrite   = true
  limit              = "rate=1024"
  storage_class      = "STANDARD"
  metadata = {
    project = "demo"
    owner   = "team-a"
  }
}

resource "ctyun_oos_object" "example-object-content" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "examples/object-with-content.txt"
  content = "Hello, OOS!"
}

output "object_source" {
  value = ctyun_oos_object.example-object-source
}

output "object_content" {
  value = ctyun_oos_object.example-object-content
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) Bucket name of the object to operate.
* `key` - (Required, ForceNew) The name of the object to operate.
* `content` - (Optional) The string value is used directly as the object content and uploaded to OOS as UTF-8 encoded text. Note: cannot be specified together with `source`.
* `source` - (Optional) The path to the local file. The file at this path is read and uploaded to OOS. Note: cannot be specified together with `content`.
* `cache_control` - (Optional) Defines the cache behavior based on the request/response.
* `content_disposition` - (Optional) Presentational information for the object.
* `content_encoding` - (Optional) Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Example values: `gzip`, `deflate`, `br`, `zstd`, or `identity`.
* `content_md5` - (Optional) The base64-encoded 128-bit MD5 digest of the message, as defined in RFC 1864. This header can be used as a message integrity check to verify that the data is the same as what was originally sent by the client.
* `content_type` - (Optional) Specifies the standard MIME type that describes the content format.
* `etag` - (Optional) The entity tag is a hash of the object. The ETag reflects changes only to the contents of an object, not its metadata.
* `forbid_overwrite` - (Optional) Specifies whether to allow overwriting an object with the same name when uploading the object. Value: `true`: do not allow overwriting an object with the same name. `false`: allow overwriting an object with the same name. If this parameter is not specified, the server defaults to `false`.
* `limit` - (Optional) Rate limit for object uploads. Value: the format is `rate=<value>`. The value is a positive integer greater than 0, in KiB/s. Values between 1 and 127 are treated as 128 KiB/s.
* `metadata` - (Optional) Any header that starts with this prefix is considered user-defined metadata. It is stored with the object and returned when the object is retrieved. The total size of PUT request headers is limited to 8 KiB. The size of user-defined metadata within PUT request headers is limited to 2 KiB.
* `storage_class` - (Optional) The storage class of the object. Value: `STANDARD`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - Resource name of the object.
* `id` - `BucketName/ObjectName`.
* `server_side_encryption` - The server-side encryption algorithm. If the server-side encryption algorithm is not configured for the bucket, this field is not returned.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import an object:

```terraform
import {
  to = ctyun_oos_object.example
  identity = {
    bucket = "example-oos-bucket"
    key    = "documents/example.txt"
  }
}
```

### Identity Schema

#### Required

* `bucket` - Name of the Bucket.
* `key` - Name of the object.

In Terraform v1.5.0 and later, use an `import` block to import an object with a slash-separated bucket and key:

```terraform
import {
  to = ctyun_oos_object.example
  id = "example-oos-bucket/documents/example.txt"
}
```

Using `terraform import`, import the object:

```console
terraform import ctyun_oos_object.example example-oos-bucket/documents/example.txt
```
