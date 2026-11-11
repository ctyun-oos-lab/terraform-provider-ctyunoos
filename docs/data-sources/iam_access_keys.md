---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_access_keys"
description: |-
  Fetch information on IAM access keys associated with the specified IAM user.
---

# Data Source: ctyun_iam_access_keys

This data source can be used to fetch information about IAM access keys of a specific IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

resource "ctyun_iam_access_key" "example-access-key" {
  user = ctyun_iam_user.example-user.name
}

data "ctyun_iam_access_keys" "example-access-keys" {
  user       = ctyun_iam_user.example-user.name
  depends_on = [ctyun_iam_access_key.example-access-key]
}

output "access_key_id_0" {
  value = tolist(data.ctyun_iam_access_keys.example-access-keys.access_keys)[0].access_key_id
}
```

## Argument Reference

This data source supports the following arguments:

* `user` - (Required) The name of the IAM user associated with the AccessKeys. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).

## Attribute Reference

This data source exports the following attributes in addition to the arguments above:

* `access_keys` - List of the IAM AccessKeys associated with the IAM user. See [`access_keys`](#access_keys) for details.
* `id` - Name of the IAM user.

### `access_keys`

* `access_key_id` - AccessKey ID.
* `create_date` - The creation time of AccessKey.
* `status` - Status of the access key. Value: `Active` (The Accesskey is active, and it can be used to call API), `Inactive` (The Accesskey is inactive, and it cannot be used to call API).
