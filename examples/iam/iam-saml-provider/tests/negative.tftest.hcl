# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-SAML-CLI-02 Invalid SAML provider inputs are rejected at plan time.
#
# terraform test expect_failures only matches Terraform checkable objects
# (variables, preconditions, check blocks). Provider ValidateFunc errors are
# not treated as expected failures, so these cases use the same rules on
# override variables.
run "rejects_invalid_provider_name" {
  command = plan

  variables {
    provider_name = "bad name"
  }

  expect_failures = [
    var.provider_name,
  ]
}

run "rejects_short_metadata_document" {
  command = plan

  variables {
    saml_metadata_document = "too-short"
  }

  expect_failures = [
    var.saml_metadata_document,
  ]
}

run "rejects_short_provider_arn" {
  command = plan

  variables {
    provider_arn = "too-short-to-be-arn"
  }

  expect_failures = [
    var.provider_arn,
  ]
}
