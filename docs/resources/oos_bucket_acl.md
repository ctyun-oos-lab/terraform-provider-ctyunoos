---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket_acl"
description: |-
  Provides an OOS bucket ACL resource.
---

# Resource: ctyun_oos_bucket_acl

Provides an OOS bucket ACL resource.

## Example Usage

```terraform
resource "ctyun_oos_bucket" "example-bucket" {
  bucket = "example-bucket"
}

resource "ctyun_oos_bucket_acl" "example-acl" {
  bucket = ctyun_oos_bucket.example-bucket.bucket
  acl    = "private"
}

output "bucket_acl" {
  value = ctyun_oos_bucket_acl.example-acl
}
```

## Argument Reference

This resource supports the following arguments:

* `bucket` - (Required, ForceNew) The name of the bucket which to apply the ACL.
* `acl` - (Optional) Sets the ACL for the bucket. Value: `private` (Only the account and IAM users/roles with corresponding permissions can read/write/delete objects in the bucket (including Put, Delete, and Get Object). Other people (including anonymous) can only access objects in the bucket if they are authorized by the bucket policy or share a link), `public-read` (Only the account and IAM users/roles with corresponding permissions can write/delete objects in the bucket (including Put and Delete Object). Anyone (including anonymous) can read objects in the bucket. This may result in unexpected access to the bucket and high costs. Please use this permission carefully), `public-read-write` (Anyone (including anonymous) can read/write/delete objects in the bucket. This may result in unexpected access to the bucket and high costs. Your legitimate rights and interests may be infringed if someone uploads illegal information. Therefore, we recommend that you do not set the bucket ACL to `public-read-write` except in special cases). If this parameter is not specified, the server defaults to `private`.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `id` - Name of the Bucket.
* `access_control_policy` - The effective access control policy, including the owner and grants returned by OOS. See [`access_control_policy`](#access_control_policy) for details.

### `access_control_policy`

* `owner` - Block that stores account information to which the bucket belongs.
* `grant` - Set of grant configuration block.

The `owner` configuration block supports the following argument:

* `display_name` - The name of the account to which the bucket belongs.
* `id` - The ID of the account to which the bucket belongs.

The `grant` configuration block supports the following argument:

* `permission` - Bucket permission.
* `grantee` - Block for the person being granted permissions.

The `grantee` configuration block supports the following argument:

* `type` - Type of grantee.
* `uri` - URI of the group to which permissions are granted. Ancestor: Grantee.

## Import

In Terraform v1.12.0 and later, an `import` block can use the identity attributes to import a bucket ACL:

```terraform
import {
  to = ctyun_oos_bucket_acl.example
  identity = {
    bucket = "example-oos-bucket"
  }
}
```

### Identity Schema

#### Required

* `bucket` - The name of the bucket.

In Terraform v1.5.0 and later, use an `import` block to import a bucket ACL by the bucket name:

```terraform
import {
  to = ctyun_oos_bucket_acl.example
  id = "example-oos-bucket"
}
```

Using `terraform import`, import the bucket ACL:

```console
terraform import ctyun_oos_bucket_acl.example example-oos-bucket
```
