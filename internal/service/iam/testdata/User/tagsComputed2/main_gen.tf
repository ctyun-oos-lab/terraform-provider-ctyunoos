# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "ctyun_iam_user" "test" {
  name = var.rName

  tags = {
    (var.unknownTagKey) = terraform_data.test.id
    (var.knownTagKey)   = var.knownTagValue
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

variable "knownTagKey" {
  type     = string
  nullable = false
}

variable "knownTagValue" {
  type     = string
  nullable = false
}
