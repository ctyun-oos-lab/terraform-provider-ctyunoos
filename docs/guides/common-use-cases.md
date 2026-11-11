---
subcategory: ""
page_title: "CTYUN OOS Provider Common Use Cases"
description: |-
  End-to-end examples that combine multiple CTYUN OOS resources into complete, ready-to-apply configurations.
---

# Common Use Cases

This guide collects end-to-end examples that combine multiple CTYUN OOS resources into complete, ready-to-apply configurations. Each section is a self-contained scenario that shows how S3, IAM, and CloudTrail resources work together, along with the data sources used to read the results back.

For the arguments and attributes of a single resource, refer to that resource's own documentation page. The examples below focus on how the resources connect.

## Managing IAM users and their permissions end to end

This scenario manages a team of IAM users with `for_each` and provisions everything Terraform can attach to each of them: an access key, a login profile, shared group membership, and a managed policy attachment. Every dependent resource iterates over the same `ctyun_iam_user.team` map, so Terraform applies them in the correct order. Two data sources read the results back: the users by name pattern, and the access keys by user.

```terraform
locals {
  user_names = {
    alpha = "app-alpha"
    bravo = "app-bravo"
  }
}

resource "ctyun_iam_user" "team" {
  for_each = local.user_names

  name          = each.value
  force_destroy = true
}

resource "ctyun_iam_access_key" "team" {
  for_each = ctyun_iam_user.team

  user   = each.value.name
  status = "Active"
}

resource "ctyun_iam_user_login_profile" "team" {
  for_each = ctyun_iam_user.team

  user                    = each.value.name
  password_reset_required = true
}

resource "ctyun_iam_group" "team" {
  name = "app-team"
}

resource "ctyun_iam_user_group_membership" "team" {
  for_each = ctyun_iam_user.team

  user  = each.value.name
  group = ctyun_iam_group.team.name
}

resource "ctyun_iam_policy" "list_users" {
  name = "app-users-read"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["iam:ListUsers"]
      Resource = "*"
    }]
  })
}

resource "ctyun_iam_user_policy_attachment" "team" {
  for_each = ctyun_iam_user.team

  user       = each.value.name
  policy_arn = ctyun_iam_policy.list_users.arn
}

data "ctyun_iam_users" "team" {
  name_regex = "^app-(alpha|bravo)$"

  depends_on = [ctyun_iam_user.team]
}

data "ctyun_iam_access_keys" "team" {
  for_each = ctyun_iam_user.team

  user = each.value.name

  depends_on = [ctyun_iam_access_key.team]
}

output "group_name" {
  value = ctyun_iam_group.team.name
}

output "returned_user_names" {
  value = data.ctyun_iam_users.team.names
}

output "access_key_statuses" {
  value = {
    for name, keys in data.ctyun_iam_access_keys.team :
    name => one([
      for access_key in keys.access_keys : access_key.status
      if access_key.access_key_id == ctyun_iam_access_key.team[name].id
    ])
  }
}
```

## OOS bucket with logging, encryption, and audit trail

This scenario configures a bucket using separate resources for each concern: access logging to a second bucket, and default server-side encryption. A bucket policy grants read access to the objects. A CloudTrail trail also delivers its audit logs to the log bucket.

```terraform
resource "ctyun_oos_bucket" "log" {
  bucket = "app-access-logs"
}

resource "ctyun_oos_bucket" "main" {
  bucket = "app-content"
}

resource "ctyun_oos_bucket_logging" "main" {
  bucket        = ctyun_oos_bucket.main.bucket
  target_bucket = ctyun_oos_bucket.log.bucket
  target_prefix = "access-logs/"
}

resource "ctyun_cloudtrail" "audit" {
  name           = "audit-trail"
  s3_bucket_name = ctyun_oos_bucket.log.bucket
  s3_key_prefix  = "audit-logs/"
}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "main" {
  bucket = ctyun_oos_bucket.main.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "ctyun_oos_bucket_policy" "main" {
  bucket = ctyun_oos_bucket.main.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = "*"
      Action    = ["oos:GetObject"]
      Resource  = "arn:ctyun:oos:::${ctyun_oos_bucket.main.bucket}/*"
    }]
  })
}

data "ctyun_cloudtrail" "audit" {
  name = ctyun_cloudtrail.audit.name
}

output "bucket_name" {
  value = ctyun_oos_bucket.main.bucket
}

output "encryption_algorithm" {
  value = ctyun_oos_bucket_server_side_encryption_configuration.main.rule[0].apply_server_side_encryption_by_default[0].sse_algorithm
}

output "trail_logging_enabled" {
  value = data.ctyun_cloudtrail.audit.enable_logging
}
```

## Copying an object within a bucket

This scenario uploads a source object and copies it to a new key with `ctyun_oos_object_copy`. The `ctyun_oos_object` data source reads the copied object back to confirm it exists.

```terraform
resource "ctyun_oos_bucket" "content" {
  bucket = "app-content"
}

resource "ctyun_oos_object" "source" {
  bucket       = ctyun_oos_bucket.content.bucket
  key          = "source.txt"
  content      = "example object body"
  content_type = "text/plain"
}

resource "ctyun_oos_object_copy" "copied" {
  bucket = ctyun_oos_bucket.content.bucket
  key    = "copied.txt"
  source = "${ctyun_oos_object.source.bucket}/${ctyun_oos_object.source.key}"
}

data "ctyun_oos_object" "copied" {
  bucket = ctyun_oos_bucket.content.bucket
  key    = ctyun_oos_object_copy.copied.key

  depends_on = [ctyun_oos_object_copy.copied]
}

output "source_etag" {
  value = ctyun_oos_object.source.etag
}

output "copied_object_etag" {
  value = data.ctyun_oos_object.copied.etag
}
```

## Cross-account management with assume-role

A **management account** (account B) manages resources in a **production account** (account A) by assuming a role account A publishes, so account A's long-lived credentials never leave that account. Each step below is a separate Terraform configuration.

### Step 1: create the operator sub-user (account B)

Create an operator user in account B that can assume roles in account A. Apply with account B's root credentials.

```terraform
variable "access_key" {
  type        = string
  description = "Account B root access key."
}

variable "secret_key" {
  type        = string
  sensitive   = true
  description = "Account B root secret key."
}

provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key
}

resource "ctyun_iam_user" "operator" {
  name = "cross-account-operator"
}

resource "ctyun_iam_access_key" "operator" {
  user = ctyun_iam_user.operator.name
}

resource "ctyun_iam_policy" "operator_assume" {
  name = "cross-account-operator-assume"

  # "*" keeps this example a single apply. In production, scope Resource to the
  # specific deploy role ARN from step 2 so the operator can assume only that role.
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "iam:AssumeRole"
      Resource = "*"
    }]
  })
}

resource "ctyun_iam_user_policy_attachment" "operator_assume" {
  user       = ctyun_iam_user.operator.name
  policy_arn = ctyun_iam_policy.operator_assume.arn
}

output "operator_arn" {
  value = ctyun_iam_user.operator.arn
}

output "operator_access_key_id" {
  value = ctyun_iam_access_key.operator.id
}

output "operator_secret_access_key" {
  value     = ctyun_iam_access_key.operator.secret
  sensitive = true
}
```

### Step 2: publish the role (account A)

Publish the deploy role in account A. Apply with account A's credentials. Set `management_principal_arn` to step 1's `operator_arn`, `account_id` to account A's own ID, and `external_id` to your own secret (reused in step 3).

```terraform
variable "access_key" {
  type        = string
  description = "Account A access key."
}

variable "secret_key" {
  type        = string
  sensitive   = true
  description = "Account A secret key."
}

variable "management_principal_arn" {
  type        = string
  description = "Operator user ARN from step 1 that is trusted to assume this role."
}

variable "account_id" {
  type        = string
  description = "Account A's own account ID, used to build the bucket ARN."
}

provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key
}

resource "ctyun_iam_role" "cross_account_deploy" {
  name = "cross-account-deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = "iam:AssumeRole"
      Principal = {
        CTYUN = var.management_principal_arn
      }
      Condition = {
        StringEquals = {
          "ctyun:ExternalId" = "ctyun-xacct-shared-secret"
        }
      }
    }]
  })
}

resource "ctyun_iam_policy" "cross_account_deploy" {
  name = "cross-account-deploy-access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["oos:*Bucket*", "oos:*Configuration"]
        Resource = "arn:ctyun:oos::${var.account_id}:deploy-artifacts"
      },
    ]
  })
}

resource "ctyun_iam_role_policy_attachment" "cross_account_deploy" {
  role       = ctyun_iam_role.cross_account_deploy.name
  policy_arn = ctyun_iam_policy.cross_account_deploy.arn
}

output "cross_account_deploy_role_arn" {
  value = ctyun_iam_role.cross_account_deploy.arn
}
```

### Step 3: assume the role and deploy (account B)

Deploy into account A through the assumed role. Apply with step 1's operator access key and step 2's role ARN.

```terraform
variable "access_key" {
  type        = string
  description = "Operator access key ID from step 1."
}

variable "secret_key" {
  type        = string
  sensitive   = true
  description = "Operator secret access key from step 1."
}

variable "cross_account_deploy_role_arn" {
  type        = string
  description = "Deploy role ARN from step 2 to assume."
}

provider "ctyun" {
  access_key = var.access_key
  secret_key = var.secret_key

  assume_role {
    role_arn     = var.cross_account_deploy_role_arn
    session_name = "terraform-cross-account"
    external_id  = "ctyun-xacct-shared-secret"
    duration     = "1h"
  }
}

resource "ctyun_oos_bucket" "deployed" {
  bucket = "deploy-artifacts"
}

output "deployed_bucket_name" {
  value = ctyun_oos_bucket.deployed.bucket
}
```
