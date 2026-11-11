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

# Empty assume_role block: skip AssumeRole and keep permanent credentials.
provider "ctyun" {
  assume_role {}
}

data "ctyun_iam_account_summary" "test" {}

output "summary_id" {
  value = data.ctyun_iam_account_summary.test.id
}

output "users" {
  value = data.ctyun_iam_account_summary.test.users
}
