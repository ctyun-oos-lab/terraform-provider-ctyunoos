---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_object_recover"
description: |-
  Provides an OOS resource for recovering objects within the soft delete retention period.
---

# Resource: ctyun_oos_object_recover

Provides an OOS resource for recovering objects within the soft delete retention period. A maximum of 1,000 objects can be restored at a time.

## Example Usage

```terraform
resource "ctyun_oos_object_recover" "example-recover-by-key" {
  bucket        = "example-bucket"
  encoding_type = "url"

  recover {
    key = "deleted/file-with spaces.txt"
  }

  recover {
    key = "deleted/second-file.txt"
  }
}

output "recover_by_key" {
  value = ctyun_oos_object_recover.example-recover-by-key
}

resource "ctyun_oos_object_recover" "example-recover-by-time-window" {
  bucket            = "example-bucket"
  soft_delete_after = 1767225600000
  #soft_delete_before = 1769904000000
}

output "recover_by_time_window" {
  value = ctyun_oos_object_recover.example-recover-by-time-window
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) Bucket name of the object to operate.
* `encoding_type` - (Optional, ForceNew) Specifies the encoding type for Key. If the key contains control characters not supported by the XML 1.0 standard, you can use this parameter to instruct the server to perform URL Encode and URL decode simultaneously. If `encoding_type = "url"` is specified, the key must be URL-encoded. Value: `url`. The characters are case insensitive.
* `soft_delete_after` - (Optional, ForceNew) Specifies recovering objects that were soft-deleted after a certain point in time. This parameter is mutually exclusive with the `recover` parameter. At least one of `recover`, `soft_delete_after`, and `soft_delete_before` must be configured. Value: Unix timestamp (UTC), time accurate to milliseconds. If both `soft_delete_after` and `soft_delete_before` exist, `soft_delete_after` must be earlier than `soft_delete_before`.
* `soft_delete_before` - (Optional, ForceNew) Specifies recovering objects that were soft-deleted before a certain point in time. This parameter is mutually exclusive with the `recover` parameter. At least one of `recover`, `soft_delete_after`, and `soft_delete_before` must be configured. Value: Unix timestamp (UTC), time accurate to milliseconds. If both `soft_delete_after` and `soft_delete_before` exist, `soft_delete_after` must be earlier than `soft_delete_before`.
* `recover` - (Optional, ForceNew) A list of objects to be recovered, supports `key`. The `recover` list must contain between 1 and 1000 items. This parameter is mutually exclusive with parameters both `soft_delete_after` and `soft_delete_before`. At least one of `recover`, `soft_delete_after`, and `soft_delete_before` must be configured. See [`recover`](#recover) for details.

### `recover`

* `key` - (Required) Specifies the name of the object to be recovered.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - `bucketName/recover/sha256_hash`.
* `recovered_keys` - The list of objects recovered successfully.
* `errors` - Information about failed object recoveries. See [`errors`](#errors) for details.

### `errors`

  * `key` - Name of the object.
  * `code` - The error code.
  * `message` - The error message.

## Import

This resource does not support import.
