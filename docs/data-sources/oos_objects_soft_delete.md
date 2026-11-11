---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_objects_soft_delete"
description: |-
  List objects within the soft delete retention period.
---

# Data Source: ctyun_oos_objects_soft_delete

This data source can be used to list objects within the soft delete retention period.

## Example Usage

```terraform
data "ctyun_oos_objects_soft_delete" "example-objects-soft-delete-data" {
  bucket             = "example-bucket"
  encoding_type      = "url"
  max_keys           = 100
  soft_delete_after  = 1757225600000
  soft_delete_before = 1769904000000
}

output "ctyun_oos_objects_soft_delete_all_parameters" {
  value = data.ctyun_oos_objects_soft_delete.example-objects-soft-delete-data
}
```

## Argument Reference

This data source supports the following arguments:

* `bucket` - (Required) The name of the bucket from which to list objects within the soft delete retention period.
* `soft_delete_after` - (Optional) Specifies listing objects that were soft-deleted after a certain point in time and are still within the soft delete retention period. Value: Unix timestamp (UTC), time accurate to milliseconds. By default, all objects within the soft delete retention period are listed. Note: `soft_delete_after` must be earlier than `soft_delete_before`.
* `soft_delete_before` - (Optional) Specifies listing objects that were soft-deleted before a certain point in time and are still within the soft delete retention period. Value: Unix timestamp (UTC), time accurate to milliseconds. By default, all objects within the soft delete retention period are listed. Note: `soft_delete_after` must be earlier than `soft_delete_before`.
* `encoding_type` - (Optional) Specifies the encoding type for Key in the response. If Key contains control characters that are not supported by the XML 1.0 standard, you can set `encoding_type` to encode the response elements in the response. Value: `url`. The characters are case insensitive.
* `max_keys` - (Optional) The maximum number of objects to return. Value: The value is an integer. If this parameter is not specified, the server defaults to 1,000.

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `contents` - The list of objects within the soft delete retention period. See [`contents`](#contents) for details.

### `contents`

* `key` - The name of the object.
* `soft_delete_at` - The time when the object was soft-deleted.
