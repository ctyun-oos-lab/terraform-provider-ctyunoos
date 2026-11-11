# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-SAML-CLI-01 Create a SAML provider, read it through both data sources, update metadata, then confirm no drift.
run "creates_and_reads_saml_provider" {
  command = apply

  assert {
    condition     = output.provider_id == output.provider_arn
    error_message = "The SAML provider resource ID should be the provider ARN."
  }

  assert {
    condition     = output.provider_name != ""
    error_message = "The SAML provider name was empty after apply."
  }

  assert {
    condition     = output.provider_uuid != ""
    error_message = "The SAML provider UUID was empty after apply."
  }

  assert {
    condition     = output.provider_valid_until != ""
    error_message = "The SAML provider valid_until was empty after apply."
  }

  assert {
    condition     = output.metadata_length >= 1000
    error_message = "The SAML metadata document was shorter than the schema minimum after apply."
  }

  assert {
    condition     = output.metadata_contains_entity_id
    error_message = "The SAML metadata document did not contain the original entity ID."
  }

  assert {
    condition     = output.data_provider_name == output.provider_name
    error_message = "The SAML provider data source name does not match the created provider."
  }

  assert {
    condition     = output.data_provider_arn == output.provider_arn
    error_message = "The SAML provider data source ARN does not match the created provider."
  }

  assert {
    condition     = output.data_provider_id == output.provider_arn
    error_message = "The SAML provider data source id should be the provider ARN."
  }

  assert {
    condition     = output.data_provider_uuid == output.provider_uuid
    error_message = "The SAML provider data source UUID does not match the resource."
  }

  assert {
    condition     = output.data_provider_valid_until == output.provider_valid_until
    error_message = "The SAML provider data source valid_until does not match the resource."
  }

  assert {
    condition     = output.data_create_date != ""
    error_message = "The SAML provider data source create_date was empty after apply."
  }

  assert {
    condition     = output.data_metadata_matches
    error_message = "The SAML provider data source metadata document does not match the resource."
  }

  assert {
    condition     = output.providers_ds_names == output.expected_provider_names
    error_message = "The SAML providers data source did not return exactly the created provider name."
  }

  assert {
    condition     = output.providers_ds_arns == output.expected_provider_arns
    error_message = "The SAML providers data source did not return exactly the created provider ARN."
  }

  assert {
    condition     = contains(output.providers_ds_uuids, output.provider_uuid)
    error_message = "The SAML providers data source did not return the created provider UUID."
  }
}

run "updates_metadata_document" {
  command = apply

  variables {
    metadata_variant = "modified"
  }

  assert {
    condition     = output.metadata_contains_entity_id
    error_message = "Updating metadata did not persist the modified entity ID."
  }

  assert {
    condition     = output.data_metadata_matches
    error_message = "The SAML provider data source metadata drifted after the update."
  }

  assert {
    condition     = output.data_provider_uuid == output.provider_uuid
    error_message = "The SAML provider UUID changed after the metadata update."
  }

  assert {
    condition     = output.providers_ds_names == output.expected_provider_names
    error_message = "The SAML providers data source name list drifted after the metadata update."
  }

  assert {
    condition     = output.providers_ds_arns == output.expected_provider_arns
    error_message = "The SAML providers data source ARN list drifted after the metadata update."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    metadata_variant = "modified"
  }

  assert {
    condition     = output.metadata_contains_entity_id
    error_message = "Apply was followed by a plan that still expected a SAML provider change."
  }
}
