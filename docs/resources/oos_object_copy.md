---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_object_copy"
description: |-
  Provides an OOS resource for copying an object.
---

# Resource: ctyun_oos_object_copy

Provides an OOS resource for copying an object.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

# The source object must exist before ctyun_oos_object_copy is created.
resource "ctyun_oos_object" "example-source-object" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  key    = "examples/copy-source.txt"
  content = "Source content for the OOS object-copy example."
}

resource "ctyun_oos_object_copy" "example-object-copy" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  key    = "examples/copied-object.txt"
  source = "${ctyun_oos_object.example-source-object.bucket}/${ctyun_oos_object.example-source-object.key}"

  cache_control          = "max-age=3600"
  content_disposition    = "attachment; filename=copied-object.txt"
  content_encoding       = "identity"
  content_type           = "text/plain"
  copy_if_match          = ctyun_oos_object.example-source-object.etag
  copy_if_modified_since = "2000-01-01T00:00:00Z"
  copy_if_none_match     = "00000000000000000000000000000000"
  copy_if_unmodified_since = "2099-12-31T23:59:59Z"
  metadata = {
    project = "demo"
    owner   = "team-a"
  }
  metadata_directive = "REPLACE"
  storage_class      = "STANDARD"
}

output "object_copy" {
  value = ctyun_oos_object_copy.example-object-copy
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) Bucket name of the object to operate.
* `key` - (Required, ForceNew) The name of the destination object.
* `source` - (Required, ForceNew) The name of the source bucket and name of the source object, separated by a slash (`/`).
* `cache_control` - (Optional) Defines the cache behavior based on the request/response.
* `content_disposition` - (Optional) Presentational information for the object.
* `content_encoding` - (Optional) Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Example values: `gzip`, `deflate`, `br`, `zstd`, or `identity`.
* `content_type` - (Optional) Specifies the standard MIME type that describes the content format.
* `copy_if_match` - (Optional) Copies the object if the entity tag (ETag) of the object matches the specified ETag, otherwise, the request returns a 412 HTTP status code error.
* `copy_if_modified_since` - (Optional) Copies the object that has been modified since the specified time, otherwise, the request will return a 412 HTTP status code error. Value: the value must be in RFC 3339 format (e.g., `2026-10-11T00:00:00Z`).
* `copy_if_none_match` - (Optional) Copies the object if the entity tag (ETag) of the object is different than the specified ETag, otherwise, the request returns a 412 HTTP status code error.
* `copy_if_unmodified_since` - (Optional) Copies the object that has not been modified since the specified time, otherwise, the request returns a 412 HTTP status code error. Value: the value must be in RFC 3339 format (e.g., `2026-10-11T00:00:00Z`).
* `metadata` - (Optional) User-defined metadata, you can customize some metadata parameters according to your needs.
* `metadata_directive` - (Optional) Specifies whether the metadata is a copy of the source object's metadata or if it is overridden by the metadata provided in the request header. If this parameter is not specified, the server defaults to `COPY`. Value: `COPY` (Other metadata is copied from the source object's metadata except for storage class and data location), `REPLACE` (All original metadata is overridden by the specified metadata). If the value is `COPY`, and the source and target objects are the same, the request must include `x-amz-storage-class`; otherwise, copying is not allowed.
* `storage_class` - (Optional) The storage class of the destination object. Value: `STANDARD`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the destination object (key).
* `etag` - The entity tag is a hash of the object. The ETag reflects changes only to the contents of an object, not its metadata.
* `arn` - Resource name of the object.
* `last_modified` - Last modified time of the object.
* `server_side_encryption` - The server-side encryption algorithm. If the server-side encryption algorithm is not configured for the bucket, this field is not returned.
* `expiration` - The time when the object expires. If the object is configured with an expiration time, this field is returned.

## Import

This resource does not support import.

## Lifecycle Notes

* In-place updates re-issue the copy. When `copy_if_match`, `copy_if_modified_since`, `copy_if_none_match`, or `copy_if_unmodified_since` is set, every apply re-issues the copy and OOS decides from the precondition; otherwise the copy is re-issued only when a relevant argument changes.
* Destroying this resource deletes the copied (destination) object; the source object is not affected.
* If the copied object is deleted outside Terraform, refresh removes it from state and the next plan proposes copying it again.
