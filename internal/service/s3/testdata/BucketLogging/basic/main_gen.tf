# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_logging" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  target_bucket = ctyun_oos_bucket.log_bucket.bucket
  target_prefix = "log/"
}

resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "${var.rName}-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName
}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
