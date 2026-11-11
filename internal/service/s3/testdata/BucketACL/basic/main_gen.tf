# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_acl" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  acl    = "private"
}

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
