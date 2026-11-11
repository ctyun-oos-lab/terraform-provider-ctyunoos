# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_user" "test" {
  name = var.rName
}

resource "ctyun_iam_policy" "test" {
  name        = var.rName
  description = "A test policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = [
        "iam:ChangePassword"
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
