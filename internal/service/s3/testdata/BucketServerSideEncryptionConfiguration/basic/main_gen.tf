# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
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
