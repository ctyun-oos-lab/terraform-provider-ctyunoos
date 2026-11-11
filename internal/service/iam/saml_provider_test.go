// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMSAMLProvider_resourceSAMLProviderValidation verifies the schema boundary and illegal-character validation
// behavior of ctyun_iam_saml_provider's name/saml_metadata_document.
//
// Positive cases: errContains is empty, meaning the ValidateFunc must not return any error.
// Negative cases: errContains lists the fields/keywords the error message must contain (matching the Schema ValidateFunc
// wording), so we assert not merely "an error occurred" but that the error hits the correct attribute and rule.
func TestUnitIAMSAMLProvider_resourceSAMLProviderValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourceSAMLProvider()
	if !res.Schema[names.AttrName].Required || !res.Schema[names.AttrName].ForceNew {
		t.Fatal("expected name to be required and ForceNew")
	}
	if !res.Schema["saml_metadata_document"].Required || res.Schema["saml_metadata_document"].ForceNew {
		t.Fatal("expected saml_metadata_document to be required and updatable")
	}

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string // empty means validation is expected to pass; non-empty means every substring must appear in the error text
	}{
		// name: validResourceName(128)
		{name: "name_valid", fieldName: names.AttrName, value: "myprovider", errContains: nil},
		{name: "name_valid_special_characters", fieldName: names.AttrName, value: "Provider_-,.+@=123", errContains: nil},
		{name: "name_valid_max_length", fieldName: names.AttrName, value: strings.Repeat("a", 128), errContains: nil},
		{name: "name_empty", fieldName: names.AttrName, value: "", errContains: []string{
			"expected length of name to be in the range (1 - 128)",
		}},
		{name: "name_too_long", fieldName: names.AttrName, value: strings.Repeat("a", 129), errContains: []string{
			"expected length of name to be in the range (1 - 128)",
		}},
		{name: "name_invalid_character", fieldName: names.AttrName, value: "invalid provider!", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},
		{name: "name_invalid_ampersand_asterisk", fieldName: names.AttrName, value: "testa&b.*+", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},

		// saml_metadata_document: the schema only validates length; the XML/SAML structure is validated by the server.
		{name: "saml_metadata_document_min", fieldName: "saml_metadata_document", value: strings.Repeat("a", 1000), errContains: nil},
		{name: "saml_metadata_document_max", fieldName: "saml_metadata_document", value: strings.Repeat("a", 10000000), errContains: nil},
		{name: "saml_metadata_document_empty", fieldName: "saml_metadata_document", value: "", errContains: []string{
			"expected length of saml_metadata_document to be in the range (1000 - 10000000)",
		}},
		{name: "saml_metadata_document_too_short", fieldName: "saml_metadata_document", value: strings.Repeat("a", 999), errContains: []string{
			"expected length of saml_metadata_document to be in the range (1000 - 10000000)",
		}},
		{name: "saml_metadata_document_too_long", fieldName: "saml_metadata_document", value: strings.Repeat("a", 10000001), errContains: []string{
			"expected length of saml_metadata_document to be in the range (1000 - 10000000)",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validate := res.Schema[tc.fieldName].ValidateFunc
			if validate == nil {
				t.Fatalf("expected %s ValidateFunc", tc.fieldName)
			}

			_, errs := validate(tc.value, tc.fieldName)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for %s length=%d, got: %s", tc.fieldName, len(tc.value), errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for %s length=%d containing %v, got none", tc.fieldName, len(tc.value), tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for %s missing %q\nfull errors: %s", tc.fieldName, want, errText)
				}
			}
		})
	}
}

// TC-IAM-SAML-01-Basic CRUD verifies creation, that a second apply of the same configuration is a Noop, and import; destruction is covered by CheckDestroy.
func TestAccIAMSAMLProvider_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityId := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	resourceName := "ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderConfig_basic(rName, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
				),
			},
			{
				Config: testAccSAMLProviderConfig_basic(rName, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-SAML-02-Metadata Update verifies that after updating the Metadata document, the remote side and state stay consistent.
func TestAccIAMSAMLProvider_metadataUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityId := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	idpEntityIdModified := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	resourceName := "ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderConfig_basic(rName, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
				),
			},
			{
				Config: testAccSAMLProviderConfig_update(rName, idpEntityIdModified),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-SAML-03-Invalid Metadata Negative sub-scenario A: passing invalid Metadata at creation is expected to return a clear error.
// The document length satisfies the schema (≥1000), but the content is invalid, hitting the CreateSAMLProvider server-side validation.
func TestAccIAMSAMLProvider_invalidMetadataCreateNegative(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccSAMLProviderConfig_metadataRaw(rName, testAccSAMLInvalidMetadataDocument()),
				ExpectError: regexache.MustCompile(`(?s)creating IAM SAML Provider.*(InvalidIdentityToken|invalidMetadata|InvalidMetadata|MalformedMetadata|MalformedDocument)`),
			},
		},
	})
}

// TC-IAM-SAML-03-Invalid Metadata Negative sub-scenario B: replacing the Metadata of an existing provider with an invalid one is expected to return a clear error.
func TestAccIAMSAMLProvider_invalidMetadataUpdateNegative(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityId := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	resourceName := "ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderConfig_basic(rName, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
				),
			},
			{
				Config:      testAccSAMLProviderConfig_metadataRaw(rName, testAccSAMLInvalidMetadataDocument()),
				ExpectError: regexache.MustCompile(`(?s)updating IAM SAML Provider.*(InvalidIdentityToken|invalidMetadata|InvalidMetadata|MalformedMetadata|MalformedDocument)`),
			},
		},
	})
}

// TC-IAM-SAML-04-Disappears verifies that after the provider is deleted remotely and refreshed, the resource is removed from state and a Create is planned.
func TestAccIAMSAMLProvider_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityId := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	resourceName := "ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderConfig_basic(rName, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceSAMLProvider(), resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderRemovedFromState(resourceName),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// Import Existing (supplementary) verifies that a SAML provider pre-created via SDK can be imported by ARN, with no perpetual diff on subsequent applies.
func TestAccIAMSAMLProvider_importExisting(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	var providerARN string
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	metadata := testAccSAMLProviderMetadataDocument(t, idpEntityID)
	resourceName := "ctyun_iam_saml_provider.test"
	config := testAccSAMLProviderConfig_basic(rName, idpEntityID)

	// A provider pre-created via SDK: if import/apply fails midway, it may not have entered Terraform state yet and
	// CheckDestroy cannot clean it up. This is a best-effort fallback deletion to avoid residue polluting later cases.
	t.Cleanup(func() {
		if providerARN == "" {
			return
		}

		cleanupCtx := context.Background()
		conn := acctest.ProviderMeta(cleanupCtx, t).IAMClient(cleanupCtx)
		_, err := tfiam.FindSAMLProviderByARN(cleanupCtx, conn, providerARN)
		if retry.NotFound(err) {
			return
		}
		if err != nil {
			t.Errorf("reading externally created IAM SAML Provider (%s) during cleanup: %s", providerARN, err)
			return
		}

		if _, err := conn.DeleteSAMLProvider(cleanupCtx, &iam.DeleteSAMLProviderInput{
			SAMLProviderArn: aws.String(providerARN),
		}); err != nil {
			t.Errorf("deleting externally created IAM SAML Provider (%s) during cleanup: %s", providerARN, err)
		}
	})

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: `data "ctyun_iam_account_summary" "bootstrap" {}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExternalCreate(ctx, t, rName, metadata, &providerARN),
				),
			},
			{
				Config:             config,
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					if providerARN == "" {
						return "", fmt.Errorf("externally created IAM SAML Provider ARN is empty")
					}
					return providerARN, nil
				},
				ImportStateCheck: testAccSAMLProviderImportStateCheck(rName, metadata),
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, rName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-SAML-05-Name ForceNew verifies that changing name triggers DestroyBeforeCreate (name is Required+ForceNew).
func TestAccIAMSAMLProvider_nameForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var out iam.GetSAMLProviderOutput
	var previousARN string
	name1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityId := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	resourceName := "ctyun_iam_saml_provider.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProviderConfig_basic(name1, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, name1),
					testAccCheckSAMLProviderCaptureARN(resourceName, &previousARN),
				),
			},
			{
				Config: testAccSAMLProviderConfig_basic(name2, idpEntityId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSAMLProviderExists(ctx, t, resourceName, &out),
					testAccCheckSAMLProviderStateMatchesCloud(resourceName, &out, name2),
					testAccCheckSAMLProviderGoneByARN(ctx, t, &previousARN),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
		},
	})
}

func testAccCheckSAMLProviderDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_saml_provider" {
				continue
			}

			_, err := tfiam.FindSAMLProviderByARN(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM SAML Provider %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckSAMLProviderExists(ctx context.Context, t *testing.T, n string, v *iam.GetSAMLProviderOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindSAMLProviderByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		if v != nil {
			*v = *output
		}

		return nil
	}
}

func testAccCheckSAMLProviderRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM SAML Provider resource (%s) is still present in state", resourceName)
		}
		return nil
	}
}

func testAccCheckSAMLProviderCaptureARN(resourceName string, arn *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", resourceName)
		}
		*arn = rs.Primary.ID
		if *arn == "" {
			return fmt.Errorf("expected SAML Provider ARN to be set")
		}
		return nil
	}
}

func testAccCheckSAMLProviderGoneByARN(ctx context.Context, t *testing.T, arn *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if arn == nil || *arn == "" {
			return fmt.Errorf("previous SAML Provider ARN is empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindSAMLProviderByARN(ctx, conn, *arn)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("IAM SAML Provider %s still exists after ForceNew replace", *arn)
	}
}

func testAccCheckSAMLProviderExternalCreate(ctx context.Context, t *testing.T, name, metadata string, providerARN *string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := conn.CreateSAMLProvider(ctx, &iam.CreateSAMLProviderInput{
			Name:                 aws.String(name),
			SAMLMetadataDocument: aws.String(metadata),
		})
		if err != nil {
			return fmt.Errorf("externally creating IAM SAML Provider (%s): %w", name, err)
		}
		if output == nil || aws.ToString(output.SAMLProviderArn) == "" {
			return fmt.Errorf("externally creating IAM SAML Provider (%s): empty ARN", name)
		}
		*providerARN = aws.ToString(output.SAMLProviderArn)
		return nil
	}
}

func testAccSAMLProviderImportStateCheck(name, metadata string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		var rs *terraform.InstanceState
		for _, state := range states {
			if state.Ephemeral.Type == "ctyun_iam_saml_provider" {
				rs = state
				break
			}
		}
		if rs == nil {
			return fmt.Errorf("expected ctyun_iam_saml_provider in import state, got %d resources", len(states))
		}
		if rs.ID == "" {
			return fmt.Errorf("expected imported SAML Provider ID to be set")
		}
		if got := rs.Attributes[names.AttrARN]; got != rs.ID {
			return fmt.Errorf("bad import arn: got %q, want %q", got, rs.ID)
		}
		if got := rs.Attributes[names.AttrName]; got != name {
			return fmt.Errorf("bad import name: got %q, want %q", got, name)
		}
		if got := rs.Attributes["saml_metadata_document"]; got != metadata {
			return fmt.Errorf("bad import metadata: imported document does not match externally created document")
		}
		if rs.Attributes["saml_provider_uuid"] == "" {
			return fmt.Errorf("expected saml_provider_uuid to be set after import")
		}
		if rs.Attributes["valid_until"] == "" {
			return fmt.Errorf("expected valid_until to be set after import")
		}
		return nil
	}
}

// testAccCheckSAMLProviderStateMatchesCloud checks after Exists that the state of the given resourceName matches the cloud resource:
// id(=arn), name, arn, saml_provider_uuid, valid_until, saml_metadata_document.
func testAccCheckSAMLProviderStateMatchesCloud(resourceName string, out *iam.GetSAMLProviderOutput, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}
		if rs.Type != "ctyun_iam_saml_provider" {
			return fmt.Errorf("%s: unexpected type %q, want ctyun_iam_saml_provider", resourceName, rs.Type)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		gotARN := rs.Primary.Attributes[names.AttrARN]
		if gotARN != rs.Primary.ID {
			return fmt.Errorf("bad id/arn: id=%s arn=%s", rs.Primary.ID, gotARN)
		}

		gotNameFromARN, err := tfiam.NameFromSAMLProviderARN(rs.Primary.ID)
		if err != nil {
			return err
		}
		if gotNameFromARN != name {
			return fmt.Errorf("bad remote name from ARN: got %s, want %s", gotNameFromARN, name)
		}
		if got := rs.Primary.Attributes[names.AttrName]; got != name {
			return fmt.Errorf("bad state name: got %s, want %s", got, name)
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

		if got, want := rs.Primary.Attributes["saml_metadata_document"], aws.ToString(out.SAMLMetadataDocument); got != want {
			return fmt.Errorf("bad metadata: state %q != cloud %q", got, want)
		}

		return nil
	}
}

func testAccSAMLProviderConfig_basic(rName, idpEntityId string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}
`, rName, idpEntityId)
}

func testAccSAMLProviderConfig_update(rName, idpEntityIdModified string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata-modified.xml.tpl", { entity_id_modified = %[2]q })
}
`, rName, idpEntityIdModified)
}

func testAccSAMLProviderConfig_metadataRaw(rName, metadata string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = %[2]q
}
`, rName, metadata)
}

func testAccSAMLProviderMetadataDocument(t *testing.T, entityID string) string {
	t.Helper()

	b, err := os.ReadFile("test-fixtures/saml-metadata.xml.tpl")
	if err != nil {
		t.Fatalf("reading SAML metadata fixture: %s", err)
	}
	return strings.ReplaceAll(string(b), "${entity_id}", entityID)
}

// testAccSAMLInvalidMetadataDocument generates a document whose length satisfies the schema (≥1000) but that cannot pass the server-side SAML parsing.
func testAccSAMLInvalidMetadataDocument() string {
	return strings.Repeat("not-a-valid-saml-metadata-document-", 40)
}
