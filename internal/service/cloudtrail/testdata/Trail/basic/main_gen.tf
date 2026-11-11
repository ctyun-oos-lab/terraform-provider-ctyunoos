# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName
}

resource "ctyun_cloudtrail" "test" {
  name           = var.rName
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
