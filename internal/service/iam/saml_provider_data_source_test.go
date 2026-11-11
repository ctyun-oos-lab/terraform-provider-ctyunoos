// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMSAMLProviderDataSource_arnValidation verifies the ValidateFunc of the ctyun_iam_saml_provider data source
// input arn (covers the former TC-IAM-DSAML-02 ARN Validation Matrix):
// only StringLenBetween(20, 2048); both valid SAML ARNs and malformed-but-length-valid ARNs pass the schema;
// too-short/too-long values fail. The ARN format is validated by the server.
func TestUnitIAMSAMLProviderDataSource_arnValidation(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceSAMLProvider()
	validateFunc := dataSource.Schema[names.AttrARN].ValidateFunc
	if validateFunc == nil {
		t.Fatal("expected arn ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{
			name:  "valid_saml_provider",
			value: "arn:ctyun:iam::abc123def456g:saml-provider/test-saml",
		},
		{
			name:  "missing_arn_prefix_passes_schema",
			value: "ctyun:iam::abc123def456g:saml-provider/test-saml",
		},
		{
			name:  "incomplete_missing_resource_passes_schema",
			value: "arn:ctyun:iam::abc123def456g:",
		},
		{
			name:  "invalid_account_id_too_short_passes_schema",
			value: "arn:ctyun:iam::123456789012:saml-provider/test-saml",
		},
		{
			name:  "invalid_account_id_uppercase_passes_schema",
			value: "arn:ctyun:iam::ABC123DEF456G:saml-provider/test-saml",
		},
		{
			name:  "invalid_account_id_too_long_passes_schema",
			value: "arn:ctyun:iam::abc123def456gh:saml-provider/test-saml",
		},
		{
			name:  "wrong_partition_aws_passes_schema",
			value: "arn:aws:iam::abc123def456g:saml-provider/test-saml",
		},
		{
			name:  "wrong_partition_aws_cn_passes_schema",
			value: "arn:aws-cn:iam::abc123def456g:saml-provider/test-saml",
		},
		{
			name:  "wrong_resource_user_passes_schema",
			value: "arn:ctyun:iam::abc123def456g:user/test-saml",
		},
		{
			name:  "wrong_resource_role_passes_schema",
			value: "arn:ctyun:iam::abc123def456g:role/test-saml",
		},
		{
			name:  "wrong_resource_policy_passes_schema",
			value: "arn:ctyun:iam::abc123def456g:policy/test-saml",
		},
		{
			name:  "malformed_passes_schema",
			value: "not-a-valid-saml-arn",
		},
		{
			name:  "too_short",
			value: "arn:ctyun:iam::1:s",
			errContains: []string{
				"expected length of arn to be in the range (20 - 2048)",
			},
		},
		{
			name:  "too_long",
			value: "arn:ctyun:iam::abc123def456g:saml-provider/" + strings.Repeat("a", 2048),
			errContains: []string{
				"expected length of arn to be in the range (20 - 2048)",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validateFunc(tc.value, names.AttrARN)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for arn=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for arn=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for arn missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TestUnitIAMSAMLProviderDataSource_schema verifies the schema shape of the ctyun_iam_saml_provider data source:
// arn is Required; it contains create_date/name/saml_metadata_document/saml_provider_uuid/valid_until; tags/tags_all are absent.
func TestUnitIAMSAMLProviderDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceSAMLProvider()

	arnSchema := dataSource.Schema[names.AttrARN]
	if arnSchema == nil {
		t.Fatal("expected arn in schema")
	}
	if !arnSchema.Required || arnSchema.Optional {
		t.Fatalf("expected arn to be Required (not Optional), got Required=%v Optional=%v", arnSchema.Required, arnSchema.Optional)
	}

	for _, name := range []string{"create_date", names.AttrName, "saml_metadata_document", "saml_provider_uuid", "valid_until"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s in schema", name)
		}
	}

	for _, name := range []string{"tags", "tags_all"} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("did not expect %s in schema", name)
		}
	}
}

// TC-IAM-DSAML-01-Lookup By ARN verifies reading by ARN after creating a SAML provider:
// the DS state fields id/arn/name/create_date/saml_metadata_document/saml_provider_uuid/valid_until
// match the cloud GetSAMLProvider result (not compared against managed resource state; CheckDestroy at the end).
func TestAccIAMSAMLProviderDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderDataSourceConfig_basic(rName, idpEntityID),
				Check:  testAccCheckSAMLProviderDataSourceStateMatchesCloud(ctx, t, dataSourceName),
			},
		},
	})
}

// TC-IAM-DSAML-02-Wrong Resource Type Negative verifies that an ARN of the wrong resource type (user) passes the
// length validation but fails at the GetSAMLProvider read stage with a parameter validation error rather than NotFound.
// The schema allows this ARN; the server is expected to return "ARN ... is not valid".
func TestAccIAMSAMLProviderDataSource_wrongResourceType(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSAMLProviderDataSourceConfig_wrongResourceType(),
				ExpectError: regexache.MustCompile(`(?s)ARN arn:ctyun:iam::abc123def456g:user/test-saml is not valid`),
			},
		},
	})
}

// TC-IAM-DSAML-03-Missing Provider Negative verifies that reading a non-existent SAML Provider ARN returns a clear
// NotFound-style error, and that no data.ctyun_iam_saml_provider remains in state (no half-written result).
func TestAccIAMSAMLProviderDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSAMLProviderDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccSAMLProviderDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckSAMLProviderDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DSAML-04-Other Account Negative verifies that reading a SAML Provider ARN that exists under another account
// returns AccessDenied from the server (providerOutsideAccount / outside your own account),
// and that no data.ctyun_iam_saml_provider remains in state (no half-written result).
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the SAML provider with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMSAMLProviderDataSource_otherAccount(t *testing.T) {
	providerARN := testAccCrossAccountSAMLProviderARN(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSAMLProviderDataSourceConfig_otherAccount(providerARN),
				ExpectError: regexache.MustCompile(`(?s)(AccessDenied|providerOutsideAccount|outside your own account|StatusCode: 403)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccSAMLProviderDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckSAMLProviderDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DSAML-05-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud GetSAMLProvider result; the second apply shows no diff (PreApply / PostApplyPostRefresh), avoiding
// perpetual plans caused by unstable read-back formats of time fields such as create_date / valid_until; CheckDestroy at the end.
func TestAccIAMSAMLProviderDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_provider.test"

	checks := testAccCheckSAMLProviderDataSourceStateMatchesCloud(ctx, t, dataSourceName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderDataSourceConfig_basic(rName, idpEntityID),
				Check:  checks,
			},
			{
				Config: testAccSAMLProviderDataSourceConfig_basic(rName, idpEntityID),
				Check:  checks,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// testAccCheckSAMLProviderDataSourceStateMatchesCloud checks that the data source state matches the cloud GetSAMLProvider
// result without relying on managed resource state: the cloud provider is fetched by the DS id (ARN), comparing
// id/arn/name/create_date/saml_metadata_document/saml_provider_uuid/valid_until.
func testAccCheckSAMLProviderDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		out, err := tfiam.FindSAMLProviderByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("reading IAM SAML Provider (%s): %w", rs.Primary.ID, err)
		}
		if out == nil {
			return fmt.Errorf("IAM SAML Provider (%s) not found in cloud", rs.Primary.ID)
		}

		wantARN := rs.Primary.ID
		if got := rs.Primary.Attributes[names.AttrARN]; got != wantARN {
			return fmt.Errorf("bad arn: got %s, want %s", got, wantARN)
		}

		wantName, err := tfiam.NameFromSAMLProviderARN(wantARN)
		if err != nil {
			return err
		}
		if got := rs.Primary.Attributes[names.AttrName]; got != wantName {
			return fmt.Errorf("bad name: got %s, want %s", got, wantName)
		}

		if out.CreateDate == nil {
			return fmt.Errorf("CreateDate is nil in cloud output")
		}
		if got, want := rs.Primary.Attributes["create_date"], aws.ToTime(out.CreateDate).Format(time.RFC3339); got != want {
			return fmt.Errorf("bad create_date: got %s, want %s", got, want)
		}

		if got, want := rs.Primary.Attributes["saml_metadata_document"], aws.ToString(out.SAMLMetadataDocument); got != want {
			return fmt.Errorf("bad saml_metadata_document: state %q != cloud %q", got, want)
		}
		if got, want := rs.Primary.Attributes["saml_provider_uuid"], aws.ToString(out.SAMLProviderUUID); got != want {
			return fmt.Errorf("bad saml_provider_uuid: got %s, want %s", got, want)
		}

		if out.ValidUntil == nil {
			return fmt.Errorf("ValidUntil is nil in cloud output")
		}
		if got, want := rs.Primary.Attributes["valid_until"], aws.ToTime(out.ValidUntil).Format(time.RFC3339); got != want {
			return fmt.Errorf("bad valid_until: got %s, want %s", got, want)
		}

		return nil
	}
}

func testAccCheckSAMLProviderDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_saml_provider.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q name=%q arn=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrName],
				rs.Primary.Attributes[names.AttrARN],
			)
		}
		return nil
	}
}

func testAccSAMLProviderDataSourceConfig_basic(rName, idpEntityID string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

data "ctyun_iam_saml_provider" "test" {
  arn = ctyun_iam_saml_provider.test.arn
}
`, rName, idpEntityID)
}

func testAccSAMLProviderDataSourceConfig_wrongResourceType() string {
	return `
data "ctyun_iam_saml_provider" "test" {
  arn = "arn:ctyun:iam::abc123def456g:user/test-saml"
}
`
}

func testAccSAMLProviderDataSourceConfig_notFound(rName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_saml_provider" "test" {
  arn = format("arn:ctyun:iam::abc123def456g:saml-provider/%%s", %[1]q)
}
`, rName)
}

func testAccSAMLProviderDataSourceConfig_otherAccount(providerARN string) string {
	return fmt.Sprintf(`
# The cross-account SAML provider is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_saml_provider" "test" {
  arn = %[1]q
}
`, providerARN)
}

func testAccSAMLProviderDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed saml provider data source read left no residual state
`
}
