---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_objects"
description: |-
  List objects.
---

# Data Source: ctyun_oos_objects

This data source can be used to list objects.

## Example Usage

The example below creates a bucket with four objects having the following keys:

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_object" "example-object-1" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "aas/prefix1/sub1/0"
  content = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

resource "ctyun_oos_object" "example-object-2" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "aas/prefix1/sub2/0"
  content = "0123456789"
}

resource "ctyun_oos_object" "example-object-3" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "aas/prefix2/0"
  content = "abcdefghijklmnopqrstuvwxyz"
}

resource "ctyun_oos_object" "example-object-4" {
  bucket  = ctyun_oos_bucket.example-bucket.bucket
  key     = "assssss/prefix2/0"
  content = "abcdefghijklmnopqrstuvwxyz"
}
```

The following three data sources query the same bucket with different argument combinations to illustrate how prefix and delimiter shape the response.

### Group keys by the first path segment

Using `delimiter = "/"` without a prefix groups object keys by their first path segment. Because every object key contains `/`, the result returns `common_prefix` values `aas/` and `assssss/` and an empty `keys` list.

```terraform
data "ctyun_oos_objects" "example-objects" {
  bucket        = ctyun_oos_bucket.example-bucket.bucket
  delimiter     = "/"
  encoding_type = "url"
  max_keys      = 100

  depends_on = [
    ctyun_oos_object.example-object-1,
    ctyun_oos_object.example-object-2,
    ctyun_oos_object.example-object-3,
    ctyun_oos_object.example-object-4,
  ]
}

output "ctyun_oos_objects_all_parameters" {
  value = data.ctyun_oos_objects.example-objects
}
```

### List all keys under a prefix

Using `prefix = "aas/"` without a delimiter returns every object key that begins with `aas/`. The result returns `keys` values `aas/prefix1/sub1/0`, `aas/prefix1/sub2/0`, and `aas/prefix2/0`, and an empty `common_prefix` list. The key `assssss/prefix2/0` is excluded because it does not match the prefix.

```terraform
data "ctyun_oos_objects" "example-objects-prefix" {
  bucket        = ctyun_oos_bucket.example-bucket.bucket
  prefix        = "aas/"
  encoding_type = "url"
  max_keys      = 100

  depends_on = [
    ctyun_oos_object.example-object-1,
    ctyun_oos_object.example-object-2,
    ctyun_oos_object.example-object-3,
    ctyun_oos_object.example-object-4,
  ]
}

output "ctyun_oos_objects_prefix" {
  value = data.ctyun_oos_objects.example-objects-prefix
}
```

### Group keys under a prefix by the next path segment

Using both `prefix = "aas/"` and `delimiter = "/"` groups object keys under `aas/` by their next path segment. The result returns `common_prefix` values `aas/prefix1/` and `aas/prefix2/`, and an empty `keys` list, letting you enumerate the next level of the key hierarchy without returning every leaf object:

```terraform
data "ctyun_oos_objects" "example-objects-prefix-delimiter" {
  bucket        = ctyun_oos_bucket.example-bucket.bucket
  delimiter     = "/"
  prefix        = "aas/"
  encoding_type = "url"
  max_keys      = 100

  depends_on = [
    ctyun_oos_object.example-object-1,
    ctyun_oos_object.example-object-2,
    ctyun_oos_object.example-object-3,
    ctyun_oos_object.example-object-4,
  ]
}

output "ctyun_oos_objects_prefix_delimiter" {
  value = data.ctyun_oos_objects.example-objects-prefix-delimiter
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket from which to list objects.
* `prefix` - (Optional) The prefix is used to limit the returned results to start with the specified prefix. The prefix can be used to divide the bucket into several groups. Defaults to empty.
* `delimiter` - (Optional) Delimiter, a character used to group keys. All keys share the same substring between the prefix and the delimiter. Value: The delimiter only supports `/`, and other delimiters are not supported.
* `encoding_type` - (Optional) Specifies the encoding type for Key in the response. If Key contains control characters that are not supported by the XML 1.0 standard, you can set `encoding_type` to encode the response elements in the response. Value: `url`. The characters are case insensitive.
* `max_keys` - (Optional) The maximum number of objects to return. Value: The value is an positive integer. If this parameter is not specified, the server defaults to 1,000.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `common_prefix` - Common prefixes that group keys whose names contain the delimiter after `prefix`. Each element is a string starting with `prefix` and ending with the next delimiter. Keys rolled up under a common prefix are not returned in `keys`. This is populated only when `delimiter` is set and matching keys exist; it is empty otherwise.
* `id` - Name of the bucket.
* `keys` - The list of keys.
