# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.5.7"

  required_providers {
    ctyun = {
      source  = "ctyun-oos-lab/ctyunoos"
      version = "~> 1.0"
    }

    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "ctyun" {}

variable "provider_name" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated SAML provider name. Used by negative tests."

  validation {
    condition = (
      var.provider_name == null ? true : (
        length(var.provider_name) >= 1 &&
        length(var.provider_name) <= 128 &&
        can(regex("^[\\w+=,.@-]+$", var.provider_name))
      )
    )
    error_message = "must match [\\w+=,.@-] and be 1-128 characters"
  }
}

variable "saml_metadata_document" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the generated SAML metadata document. Used by negative tests."

  validation {
    condition = (
      var.saml_metadata_document == null ? true : (
        length(var.saml_metadata_document) >= 1000 &&
        length(var.saml_metadata_document) <= 10000000
      )
    )
    error_message = "expected length of saml_metadata_document to be in the range (1000 - 10000000)"
  }
}

variable "provider_arn" {
  type        = string
  default     = null
  nullable    = true
  description = "Override the SAML provider data source ARN. Used by negative tests."

  validation {
    condition = (
      var.provider_arn == null ? true : (
        length(var.provider_arn) >= 20 &&
        length(var.provider_arn) <= 2048
      )
    )
    error_message = "expected length of arn to be in the range (20 - 2048)"
  }
}

variable "metadata_variant" {
  type        = string
  default     = "original"
  description = "Which SAML metadata fixture to apply. Valid values are original and modified."

  validation {
    condition     = contains(["original", "modified"], var.metadata_variant)
    error_message = "metadata_variant must be original or modified"
  }
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  provider_name = coalesce(var.provider_name, "tf-acc-test-${random_id.suffix.hex}-saml")
  entity_id     = "https://tf-acc-test-${random_id.suffix.hex}.example.test"
  entity_id_mod = "https://tf-acc-test-${random_id.suffix.hex}-mod.example.test"
  active_entity_id = (
    var.metadata_variant == "original" ? local.entity_id : local.entity_id_mod
  )
  generated_metadata = (
    var.metadata_variant == "original"
    ? templatefile("${path.module}/saml-metadata.xml.tpl", { entity_id = local.entity_id })
    : templatefile("${path.module}/saml-metadata-modified.xml.tpl", { entity_id_modified = local.entity_id_mod })
  )
}

resource "ctyun_iam_saml_provider" "test" {
  name                   = local.provider_name
  saml_metadata_document = coalesce(var.saml_metadata_document, local.generated_metadata)
}

data "ctyun_iam_saml_provider" "test" {
  arn = coalesce(var.provider_arn, ctyun_iam_saml_provider.test.arn)
}

data "ctyun_iam_saml_providers" "test" {
  name_regex = "^${local.provider_name}$"

  depends_on = [ctyun_iam_saml_provider.test]
}

output "provider_id" {
  value = ctyun_iam_saml_provider.test.id
}

output "provider_arn" {
  value = ctyun_iam_saml_provider.test.arn
}

output "provider_name" {
  value = ctyun_iam_saml_provider.test.name
}

output "provider_uuid" {
  value = ctyun_iam_saml_provider.test.saml_provider_uuid
}

output "provider_valid_until" {
  value = ctyun_iam_saml_provider.test.valid_until
}

output "metadata_length" {
  value = length(ctyun_iam_saml_provider.test.saml_metadata_document)
}

output "metadata_contains_entity_id" {
  value = strcontains(ctyun_iam_saml_provider.test.saml_metadata_document, local.active_entity_id)
}

output "data_provider_id" {
  value = data.ctyun_iam_saml_provider.test.id
}

output "data_provider_arn" {
  value = data.ctyun_iam_saml_provider.test.arn
}

output "data_provider_name" {
  value = data.ctyun_iam_saml_provider.test.name
}

output "data_provider_uuid" {
  value = data.ctyun_iam_saml_provider.test.saml_provider_uuid
}

output "data_provider_valid_until" {
  value = data.ctyun_iam_saml_provider.test.valid_until
}

output "data_create_date" {
  value = data.ctyun_iam_saml_provider.test.create_date
}

output "data_metadata_matches" {
  value = data.ctyun_iam_saml_provider.test.saml_metadata_document == ctyun_iam_saml_provider.test.saml_metadata_document
}

output "expected_provider_arns" {
  value = toset([ctyun_iam_saml_provider.test.arn])
}

output "expected_provider_names" {
  value = toset([local.provider_name])
}

output "providers_ds_arns" {
  value = data.ctyun_iam_saml_providers.test.arns
}

output "providers_ds_names" {
  value = data.ctyun_iam_saml_providers.test.names
}

output "providers_ds_uuids" {
  value = toset(data.ctyun_iam_saml_providers.test.providers[*].saml_provider_uuid)
}
