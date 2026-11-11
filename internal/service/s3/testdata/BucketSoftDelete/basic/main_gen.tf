# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  days   = 7
}

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName

}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
