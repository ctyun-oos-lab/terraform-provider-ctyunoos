# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  rule {
    id     = var.rName
    prefix = "prefix/"
    status = "Enabled"

    expiration {
      days = 365
    }
  }
}

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
