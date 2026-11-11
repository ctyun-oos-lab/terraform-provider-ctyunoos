# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }
  }
}

provider "ctyun" {}

variable "user_name" {
  type        = string
  description = "Existing IAM user name to import by ID."
}

import {
  to = ctyun_iam_user.test
  id = var.user_name
}

resource "ctyun_iam_user" "test" {
  name = var.user_name
}
