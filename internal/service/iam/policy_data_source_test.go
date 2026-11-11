// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMPolicyDataSource_schema verifies the schema shape of the ctyun_iam_policy data source:
// it must not contain the upstream AWS fields is_attachable/update_date; arn/name are both Optional+Computed
// with ExactlyOneOf=[arn, name]; arn has a ValidateFunc.
func TestUnitIAMPolicyDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourcePolicy()

	if _, ok := dataSource.Schema["is_attachable"]; ok {
		t.Fatalf("unexpected is_attachable schema")
	}

	if _, ok := dataSource.Schema["update_date"]; ok {
		t.Fatalf("unexpected update_date schema")
	}

	wantExactlyOneOf := []string{names.AttrARN, names.AttrName}

	arnSchema := dataSource.Schema[names.AttrARN]
	if !arnSchema.Optional || !arnSchema.Computed {
		t.Fatalf("expected arn to be optional and computed")
	}
	if !reflect.DeepEqual(arnSchema.ExactlyOneOf, wantExactlyOneOf) {
		t.Fatalf("expected arn ExactlyOneOf=%v, got %v", wantExactlyOneOf, arnSchema.ExactlyOneOf)
	}
	if arnSchema.ValidateFunc == nil {
		t.Fatal("expected arn ValidateFunc")
	}

	nameSchema := dataSource.Schema[names.AttrName]
	if !nameSchema.Optional || !nameSchema.Computed {
		t.Fatalf("expected name to be optional and computed")
	}
	if !reflect.DeepEqual(nameSchema.ExactlyOneOf, wantExactlyOneOf) {
		t.Fatalf("expected name ExactlyOneOf=%v, got %v", wantExactlyOneOf, nameSchema.ExactlyOneOf)
	}
}

// TestUnitIAMPolicyDataSource_arnValidation verifies the ValidateFunc of the ctyun_iam_policy data source input arn:
// only StringLenBetween(20, 2048); both valid policy ARNs and malformed-but-length-valid ARNs pass the schema;
// too-short/too-long values fail. The ARN format is validated by the server.
func TestUnitIAMPolicyDataSource_arnValidation(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourcePolicy()
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
			name:  "valid_local",
			value: "arn:ctyun:iam::32fefj64y54gc:policy/test_policy",
		},
		{
			name:  "valid_oos",
			value: "arn:ctyun:iam::OOS:policy/IAMReadOnlyAccess",
		},
		{
			name:  "wrong_resource_role_passes_schema",
			value: "arn:ctyun:iam::32fefj64y54gc:role/test",
		},
		{
			name:  "wrong_resource_user_passes_schema",
			value: "arn:ctyun:iam::32fefj64y54gc:user/test",
		},
		{
			name:  "wrong_partition_aws_passes_schema",
			value: "arn:aws:iam::32fefj64y54gc:policy/test",
		},
		{
			name:  "malformed_missing_arn_prefix_passes_schema",
			value: "ctyun:iam::32fefj64y54gc:policy/test",
		},
		{
			name:  "malformed_passes_schema",
			value: "not-a-valid-policy-arn",
		},
		{
			name:  "too_short",
			value: "arn:ctyun:iam::1:p",
			errContains: []string{
				"expected length of arn to be in the range (20 - 2048)",
			},
		},
		{
			name:  "too_long",
			value: "arn:ctyun:iam::32fefj64y54gc:policy/" + strings.Repeat("a", 2048),
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

// TC-IAM-DPOLICY-05-ARN Name Conflict Validation verifies the ExactlyOneOf constraint on arn and name:
// setting neither or both fails at the schema stage; the success paths with only arn / only name are covered by TC-01/02.
func TestUnitIAMPolicyDataSource_exactlyOneOf(t *testing.T) {
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyARN := fmt.Sprintf("arn:ctyun:iam::32fefj64y54gc:policy/%s", policyName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyDataSourceConfig_neither(),
				ExpectError: regexache.MustCompile(`(?s)(Invalid combination of arguments|one of .* must be specified|"arn":|"name":)`),
			},
			{
				Config:      testAccPolicyDataSourceConfig_both(policyARN, policyName),
				ExpectError: regexache.MustCompile(`(?s)(Invalid combination of arguments|only one of .* can be specified|"arn":|"name":)`),
			},
		},
	})
}

// TestUnitIAMPolicyDataSource_findPolicyARNByName verifies the three branches of exact-matching a policy ARN by name:
// 0 matches → no IAM policy found; 1 match → returns that ARN; multiple with the same name → multiple IAM policies found.
func TestUnitIAMPolicyDataSource_findPolicyARNByName(t *testing.T) {
	t.Parallel()

	policies := []awstypes.Policy{
		{
			Arn:        aws.String("arn:ctyun:iam::111:policy/other"),
			PolicyName: aws.String("other"),
		},
		{
			Arn:        aws.String("arn:ctyun:iam::111:policy/target-a"),
			PolicyName: aws.String("target"),
		},
		{
			Arn:        aws.String("arn:ctyun:iam::111:policy/target-b"),
			PolicyName: aws.String("target"),
		},
	}

	t.Run("zero", func(t *testing.T) {
		t.Parallel()
		_, err := tfiam.FindPolicyARNByNameFromList(policies, "missing")
		if err == nil || !strings.Contains(err.Error(), "no IAM policy found") {
			t.Fatalf("expected no IAM policy found, got %v", err)
		}
	})

	t.Run("one", func(t *testing.T) {
		t.Parallel()
		got, err := tfiam.FindPolicyARNByNameFromList(policies, "other")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "arn:ctyun:iam::111:policy/other"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()
		_, err := tfiam.FindPolicyARNByNameFromList(policies, "target")
		if err == nil || !strings.Contains(err.Error(), "multiple IAM policies found") {
			t.Fatalf("expected multiple IAM policies found, got %v", err)
		}
	})
}

// TestUnitIAMPolicyDataSource_flattenPolicy verifies that flattenPolicy maps an API Policy to
// arn/attachment_count/description/name/policy_id; nil input returns nil.
func TestUnitIAMPolicyDataSource_flattenPolicy(t *testing.T) {
	t.Parallel()

	if got := tfiam.FlattenPolicy(nil); got != nil {
		t.Fatalf("expected nil for nil policy, got %#v", got)
	}

	got := tfiam.FlattenPolicy(&awstypes.Policy{
		Arn:             aws.String("arn:ctyun:iam::969618309127:policy/example"),
		AttachmentCount: aws.Int32(2),
		Description:     aws.String("desc"),
		PolicyId:        aws.String("ANPAEXAMPLE"),
		PolicyName:      aws.String("example"),
	})

	want := map[string]any{
		names.AttrARN:         "arn:ctyun:iam::969618309127:policy/example",
		"attachment_count":    2,
		names.AttrDescription: "desc",
		names.AttrName:        "example",
		"policy_id":           "ANPAEXAMPLE",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected flattenPolicy result:\ngot  %#v\nwant %#v", got, want)
	}
}

// TestUnitIAMPolicyDataSource_resourcePolicyFlattenPolicyDocument verifies
// resourcePolicyFlattenPolicyDocument: after URL decoding, the document is normalized via PolicyToSet and written to policy;
// when an equivalent policy already exists, the existing normalized value is preserved.
func TestUnitIAMPolicyDataSource_resourcePolicyFlattenPolicyDocument(t *testing.T) {
	t.Parallel()

	const rawDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"oos:ListBuckets","Resource":"*"}]}`
	encodedDoc := url.QueryEscape(rawDoc)

	t.Run("urlDecodeAndNormalize", func(t *testing.T) {
		t.Parallel()

		d := schema.TestResourceDataRaw(t, tfiam.DataSourcePolicy().Schema, map[string]any{})
		d.SetId("arn:ctyun:iam::111:policy/example")

		if err := tfiam.ResourcePolicyFlattenPolicyDocument(encodedDoc, d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := d.Get(names.AttrPolicy).(string)
		want, err := verify.PolicyToSet("", rawDoc)
		if err != nil {
			t.Fatalf("PolicyToSet: %v", err)
		}
		if got != want {
			t.Fatalf("expected policy %q, got %q", want, got)
		}
	})

	t.Run("policyToSetKeepsEquivalentExisting", func(t *testing.T) {
		t.Parallel()

		existing, err := verify.PolicyToSet("", rawDoc)
		if err != nil {
			t.Fatalf("PolicyToSet: %v", err)
		}

		d := schema.TestResourceDataRaw(t, tfiam.DataSourcePolicy().Schema, map[string]any{
			names.AttrPolicy: existing,
		})
		d.SetId("arn:ctyun:iam::111:policy/example")

		// A cloud document that is semantically equivalent but differs in whitespace should keep the normalized existing value.
		cloudDoc := `{ "Version" : "2012-10-17", "Statement" : [ { "Effect" : "Allow", "Action" : "oos:ListBuckets", "Resource" : "*" } ] }`
		if err := tfiam.ResourcePolicyFlattenPolicyDocument(url.QueryEscape(cloudDoc), d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := d.Get(names.AttrPolicy).(string); got != existing {
			t.Fatalf("expected existing policy preserved, got %q want %q", got, existing)
		}
	})
}

// TC-IAM-DPOLICY-01-Lookup By ARN verifies that after reading a policy by arn, the data source state fields
// description/policy/policy_id/arn/name/create_date/attachment_count/scope match the cloud GetPolicy result
// (not compared against managed resource state; the configuration may use ctyun_iam_policy as fixture; CheckDestroy at the end confirms the policy is destroyed).
func TestAccIAMPolicyDataSource_arn(t *testing.T) {
	ctx := acctest.Context(t)
	datasourceName := "data.ctyun_iam_policy.test"
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyDataSourceConfig_arn(policyName),
				Check:  testAccCheckPolicyDataSourceStateMatchesCloud(ctx, t, datasourceName),
			},
		},
	})
}

// TC-IAM-DPOLICY-02-Lookup By Name verifies that after reading the same policy by name, the data source state
// matches the cloud GetPolicy result (same field set as TC-01; not compared against managed resource state; CheckDestroy at the end).
func TestAccIAMPolicyDataSource_name(t *testing.T) {
	ctx := acctest.Context(t)
	datasourceName := "data.ctyun_iam_policy.test"
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyDataSourceConfig_name(policyName),
				Check:  testAccCheckPolicyDataSourceStateMatchesCloud(ctx, t, datasourceName),
			},
		},
	})
}

// TC-IAM-DPOLICY-03-Mutual Lookup Stability verifies alternating arn and name lookups on the same policy
// (arn → name → arn): at each step the data source state matches the cloud; subsequent read-backs show no drift relative
// to the first snapshot (id/arn/name/description/policy_id/attachment_count/create_date/scope/policy, with policy compared
// for semantic equivalence); CheckDestroy at the end.
func TestAccIAMPolicyDataSource_mutualLookup(t *testing.T) {
	ctx := acctest.Context(t)
	datasourceName := "data.ctyun_iam_policy.test"
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	var snapshot testAccPolicyDataSourceSnapshot

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyDataSourceConfig_arn(policyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyDataSourceStateMatchesCloud(ctx, t, datasourceName),
					testAccCheckPolicyDataSourceCaptureSnapshot(datasourceName, &snapshot),
				),
			},
			{
				Config: testAccPolicyDataSourceConfig_name(policyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyDataSourceStateMatchesCloud(ctx, t, datasourceName),
					testAccCheckPolicyDataSourceMatchesSnapshot(datasourceName, &snapshot),
				),
			},
			{
				Config: testAccPolicyDataSourceConfig_arn(policyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyDataSourceStateMatchesCloud(ctx, t, datasourceName),
					testAccCheckPolicyDataSourceMatchesSnapshot(datasourceName, &snapshot),
				),
			},
		},
	})
}

// TC-IAM-DPOLICY-04-Missing Policy Negative verifies that querying with a non-existent name or non-existent ARN returns a
// clear error: the name path resolves via ListPolicies and yields "no IAM policy found" when the match count is 0; the ARN
// path goes through GetPolicy and expects NoSuchEntity/404 etc.
func TestAccIAMPolicyDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	missingName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingARN := fmt.Sprintf("arn:ctyun:iam::32fefj64y54gc:policy/%s", missingName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyDataSourceConfig_nameOnly(missingName),
				ExpectError: regexache.MustCompile(`(?s)no IAM policy found`),
			},
			{
				Config:      testAccPolicyDataSourceConfig_arnOnly(missingARN),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
		},
	})
}

// TC-IAM-DPOLICY-06-Invalid ARN Negative verifies that a malformed-but-length-valid arn passes the length validation and
// fails at the GetPolicy read stage with a parameter validation error rather than NotFound.
// The schema allows this ARN; the server is expected to return "ARN not-a-valid-policy-arn is not valid".
func TestAccIAMPolicyDataSource_invalidARN(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyDataSourceConfig_arnOnly("not-a-valid-policy-arn"),
				ExpectError: regexache.MustCompile(`(?s)ARN not-a-valid-policy-arn is not valid`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccPolicyDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckPolicyDataSourceAbsentFromState(),
			},
		},
	})
}

// testAccCheckPolicyDataSourceStateMatchesCloud checks that the data source state matches the cloud GetPolicy result without
// relying on managed resource state: the cloud policy is fetched by the DS id/ARN, comparing
// id/arn/name/description/policy_id/attachment_count/create_date/scope/policy.
func testAccCheckPolicyDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, policyDocument, scope, err := tfiam.FindPolicyDetailsByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}
		if policy == nil {
			return fmt.Errorf("IAM Policy (%s) not found in cloud", rs.Primary.ID)
		}

		wantARN := aws.ToString(policy.Arn)
		if rs.Primary.ID != wantARN {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, wantARN)
		}
		if got := rs.Primary.Attributes[names.AttrARN]; got != wantARN {
			return fmt.Errorf("bad arn: got %s, want %s", got, wantARN)
		}
		if got, want := rs.Primary.Attributes[names.AttrName], aws.ToString(policy.PolicyName); got != want {
			return fmt.Errorf("bad name: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrDescription], aws.ToString(policy.Description); got != want {
			return fmt.Errorf("bad description: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["policy_id"], aws.ToString(policy.PolicyId); got != want {
			return fmt.Errorf("bad policy_id: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["attachment_count"], strconv.Itoa(int(aws.ToInt32(policy.AttachmentCount))); got != want {
			return fmt.Errorf("bad attachment_count: got %s, want %s", got, want)
		}
		if policy.CreateDate == nil {
			return fmt.Errorf("CreateDate is nil in cloud output")
		}
		if got, want := rs.Primary.Attributes["create_date"], aws.ToTime(policy.CreateDate).Format(time.RFC3339); got != want {
			return fmt.Errorf("bad create_date: got %s, want %s", got, want)
		}
		if scope == "" {
			return fmt.Errorf("GetPolicy response did not include scope")
		}
		if got := rs.Primary.Attributes[names.AttrScope]; got != scope {
			return fmt.Errorf("bad scope: got %s, want %s", got, scope)
		}

		policyDocument, err = url.QueryUnescape(policyDocument)
		if err != nil {
			return fmt.Errorf("parsing IAM Policy (%s) document: %w", wantARN, err)
		}
		statePolicy := rs.Primary.Attributes[names.AttrPolicy]
		if !verify.PolicyStringsEquivalent(statePolicy, policyDocument) {
			return fmt.Errorf("bad policy document: got %s, want %s", statePolicy, policyDocument)
		}

		return nil
	}
}

// testAccPolicyDataSourceSnapshot saves the stable fields read back by the data source for drift comparison across alternating arn/name lookups.
type testAccPolicyDataSourceSnapshot struct {
	id              string
	arn             string
	name            string
	description     string
	policyID        string
	attachmentCount string
	createDate      string
	scope           string
	policy          string
}

func testAccCheckPolicyDataSourceCaptureSnapshot(dataSourceName string, snapshot *testAccPolicyDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		*snapshot = testAccPolicyDataSourceSnapshot{
			id:              rs.Primary.ID,
			arn:             rs.Primary.Attributes[names.AttrARN],
			name:            rs.Primary.Attributes[names.AttrName],
			description:     rs.Primary.Attributes[names.AttrDescription],
			policyID:        rs.Primary.Attributes["policy_id"],
			attachmentCount: rs.Primary.Attributes["attachment_count"],
			createDate:      rs.Primary.Attributes["create_date"],
			scope:           rs.Primary.Attributes[names.AttrScope],
			policy:          rs.Primary.Attributes[names.AttrPolicy],
		}
		return nil
	}
}

func testAccCheckPolicyDataSourceMatchesSnapshot(dataSourceName string, snapshot *testAccPolicyDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if snapshot == nil || snapshot.id == "" {
			return fmt.Errorf("policy data source snapshot is empty")
		}

		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		if got, want := rs.Primary.ID, snapshot.id; got != want {
			return fmt.Errorf("drift id: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], snapshot.arn; got != want {
			return fmt.Errorf("drift arn: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrName], snapshot.name; got != want {
			return fmt.Errorf("drift name: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrDescription], snapshot.description; got != want {
			return fmt.Errorf("drift description: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["policy_id"], snapshot.policyID; got != want {
			return fmt.Errorf("drift policy_id: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["attachment_count"], snapshot.attachmentCount; got != want {
			return fmt.Errorf("drift attachment_count: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["create_date"], snapshot.createDate; got != want {
			return fmt.Errorf("drift create_date: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrScope], snapshot.scope; got != want {
			return fmt.Errorf("drift scope: got %s, want %s", got, want)
		}
		gotPolicy := rs.Primary.Attributes[names.AttrPolicy]
		if !verify.PolicyStringsEquivalent(gotPolicy, snapshot.policy) {
			return fmt.Errorf("drift policy document: got %s, want %s", gotPolicy, snapshot.policy)
		}

		return nil
	}
}

func testAccPolicyBaseDataSourceConfig(policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %q
  description = "My test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:Describe*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}`, policyName)
}

func testAccPolicyDataSourceConfig_arn(policyName string) string {
	return acctest.ConfigCompose(
		testAccPolicyBaseDataSourceConfig(policyName), `
data "ctyun_iam_policy" "test" {
  arn = ctyun_iam_policy.test.arn
}
`)
}

func testAccPolicyDataSourceConfig_name(policyName string) string {
	return acctest.ConfigCompose(
		testAccPolicyBaseDataSourceConfig(policyName), `
data "ctyun_iam_policy" "test" {
  name = ctyun_iam_policy.test.name
}
`)
}

func testAccPolicyDataSourceConfig_nameOnly(policyName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_policy" "test" {
  name = %[1]q
}
`, policyName)
}

func testAccPolicyDataSourceConfig_arnOnly(policyARN string) string {
	return fmt.Sprintf(`
data "ctyun_iam_policy" "test" {
  arn = %[1]q
}
`, policyARN)
}

func testAccPolicyDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed policy read left no residual state
`
}

func testAccCheckPolicyDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_policy.") {
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

func testAccPolicyDataSourceConfig_neither() string {
	return `
data "ctyun_iam_policy" "test" {
}
`
}

func testAccPolicyDataSourceConfig_both(policyARN, policyName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_policy" "test" {
  arn  = %[1]q
  name = %[2]q
}
`, policyARN, policyName)
}
