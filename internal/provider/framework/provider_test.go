// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package framework

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestUnitFrameworkProvider_AssumeRoleWithWebIdentitySchemaAbsent(t *testing.T) {
	t.Parallel()

	p := &frameworkProvider{}
	response := provider.SchemaResponse{}
	p.Schema(t.Context(), provider.SchemaRequest{}, &response)

	if _, ok := response.Schema.Blocks["assume_role_with_web_identity"]; ok {
		t.Fatal("provider schema must not expose assume_role_with_web_identity")
	}
}

func TestUnitFrameworkProvider_TagSchemasAbsent(t *testing.T) {
	t.Parallel()

	p := &frameworkProvider{}
	response := provider.SchemaResponse{}
	p.Schema(t.Context(), provider.SchemaRequest{}, &response)

	for _, name := range []string{"default_tags", "ignore_tags", "tag_policy_compliance"} {
		if _, ok := response.Schema.Attributes[name]; ok {
			t.Fatalf("provider schema must not expose %q", name)
		}
		if _, ok := response.Schema.Blocks[name]; ok {
			t.Fatalf("provider schema must not expose %q", name)
		}
	}
}

func TestUnitFrameworkProvider_RemovedAWSConfigurationSchemasAbsent(t *testing.T) {
	t.Parallel()

	p := &frameworkProvider{}
	response := provider.SchemaResponse{}
	p.Schema(t.Context(), provider.SchemaRequest{}, &response)

	for _, name := range []string{
		"profile",
		"shared_config_files",
		"ctyunoos_endpoint",
		"ctyunoos_endpoint_iam",
		"ctyunoos_endpoint_trail",
		"shared_credentials_files",
		"skip_credentials_validation",
		"skip_requesting_account_id",
	} {
		if _, ok := response.Schema.Attributes[name]; ok {
			t.Fatalf("provider schema must not expose %q", name)
		}
	}
}

func TestUnitFrameworkProvider_RetainedTransportAndRetrySchemasPresent(t *testing.T) {
	t.Parallel()

	p := &frameworkProvider{}
	response := provider.SchemaResponse{}
	p.Schema(t.Context(), provider.SchemaRequest{}, &response)

	for _, name := range []string{
		"custom_ca_bundle",
		"http_proxy",
		"https_proxy",
		"insecure",
		"max_retries",
		"no_proxy",
		"retry_mode",
		"s3_use_path_style",
		"token_bucket_rate_limiter_capacity",
	} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("provider schema must expose %q", name)
		}
	}
}
