---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_object"
description: |-
  Fetch an object or its metadata.
---

# Data Source: ctyun_oos_object

This data source can be used to fetch an object or its metadata. When fetching metadata, the object itself is not returned.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_object" "example-object" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "example-object"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

data "ctyun_oos_object" "example-object-data" {
  bucket        = ctyun_oos_object.example-object.bucket
  key           = ctyun_oos_object.example-object.key
  download_body = true
  range         = "bytes=0-9"
}

output "ctyun_oos_object_all_parameters" {
  value = data.ctyun_oos_object.example-object-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket from which to retrieve objects.
* `key` - (Required) The absolute path of the object.
* `download_body` - (Optional) Specifies whether to return `body_base64`, which contains the raw content of the object. Value: `true` (Returns `body_base64` containing the raw content of the object. If the object's ContentType is a text type (text/plain), the raw content is returned in `body`), `false` (Neither `body_base64` nor `body` is returned). When unset, `body_base64` is not returned. If the object's ContentType is a text type (text/plain), the raw content is returned in `body`.
* `range` - (Optional) Specifies the length of the object to retrieve. For example: `range = "bytes=0-9"`.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - BucketName/ObjectName.
* `arn` - Resource name of the object.
* `body` - The original content of the object.
* `body_base64` - The Base64 encoding of the object's original content.
* `cache_control` - Caching behavior along the request/reply chain.
* `content_disposition` - The Content-Disposition response header.
* `content_encoding` - Content encodings that have been applied to the object.
* `content_length` - Size of the object, in bytes.
* `content_type` - The Content-Type response header.
* `etag` - The entity tag is a hash of the object. The ETag reflects changes only to the contents of an object, not its metadata.
* `expiration` - If the object is configured with an expiration time, then OOS returns this parameter.
* `expires` - Date and time at which the object is no longer cacheable.
* `last_modified` - Last modified time of the object.
* `server_side_encryption` - The server-side encryption algorithm. If the server-side encryption algorithm is not configured for the Bucket, this field is not returned.
* `metadata` - User-defined metadata.
* `storage_class` - The storage class of the object.
* `data_location` - The data location of the object.
* `metadata_location` - The metadata location of the object.
