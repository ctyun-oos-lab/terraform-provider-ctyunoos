# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid       = "*"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
        Action    = "oos:*"
        Resource  = "arn:ctyun:oos:::${var.rName}/*"
      }
    ]
  })
}

resource "ctyun_oos_bucket" "test" {
  bucket = var.rName

}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
