# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_iam_policy" "test" {
  name = var.rName

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = [
        "ec2:Describe*"
      ]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}
variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
