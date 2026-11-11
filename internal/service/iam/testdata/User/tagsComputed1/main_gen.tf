# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_iam_user" "test" {
  name = var.rName

  tags = {
    (var.unknownTagKey) = terraform_data.test.id
  }
}

resource "terraform_data" "test" {}

variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}

variable "unknownTagKey" {
  type     = string
  nullable = false
}
