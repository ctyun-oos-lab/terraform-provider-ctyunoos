---
subcategory: "IAM (Identity & Access Management)"
page_title: "CTYUN OOS: ctyun_iam_access_key"
description: |-
  Provides an IAM access key.
---

# Resource: ctyun_iam_access_key

Provides an IAM resource to manage AccessKey for the specified IAM user.

## Example Usage

```terraform
resource "ctyun_iam_user" "example-user" {
  name = "example-user"
}

resource "ctyun_iam_access_key" "example-access-key" {
  user    = ctyun_iam_user.example-user.name
  status  = "Active"
  pgp_key = "keybase:hashicorp"
}
```

## Argument Reference

This resource supports the following arguments:

* `user` - (Required, ForceNew) The name of the IAM user. The value is a string of 1 to 64 insensitive letters, numbers, or special characters. The special characters only include underscores (`_`), hyphens (`-`), comma (`,`), period (`.`), plus sign (`+`), equal sign (`=`), and at sign (`@`).
* `status` - (Optional) The status of the AccessKey. Values: `Active` (The AccessKey is active, and it can be used to call API), `Inactive` (The AccessKey is inactive, and it cannot be used to call API).
* `pgp_key` - (Optional, ForceNew) For use by the encrypted_secret output attribute. The value can be a Base64-encoded PGP public key, or a Keybase username in the format `keybase:some_person_that_exists`. If providing a Base64-encoded PGP public key, provide the "raw" version instead of the "armored" version (for example, avoid passing the `-a` option to the `gpg --export` command).

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - AccessKey ID.
* `create_date` - The creation time of AccessKey.
* `encrypted_secret` - If `pgp_key` is specified, the encrypted_secret attribute will contain a Base64-encoded encrypted ciphertext. This attribute is not available for imported resources. You can decrypt the encrypted ciphertext using the command `terraform output -raw encrypted_secret | base64 –decode | keybase pgp decypt`.
* `key_fingerprint` - The fingerprint of the PGP key used to encrypt the secret. This attribute is not available for imported resources.
* `last_used_date` - The last time the specified AccessKey was used.
* `secret` - The key used to sign the request.
* `service_name` - The service name the specified AccessKey was used.

## Import

In Terraform v1.5.0 and later, use an `import` block to import an IAM access key by its access key ID:

```terraform
import {
  to = ctyun_iam_access_key.example-access-key
  id = "abc1234567890abcdefa"
}
```

Using `terraform import`, import an IAM access key by its access key ID:

```console
terraform import ctyun_iam_access_key.example-access-key abc1234567890abcdefa
```

The OOS IAM service does not return the access key secret, PGP key, encrypted secret, or key fingerprint during import.
