// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMUserPolicyAttachmentsDataSource_schema verifies the schema shape of the ctyun_iam_user_policy_attachments data source:
// it contains attached_policies/id/user_name; attached_policies is Computed; user_name is Required and has Validators.
func TestUnitIAMUserPolicyAttachmentsDataSource_schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewUserPolicyAttachmentsDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var resp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	for _, name := range []string{"attached_policies", names.AttrID, names.AttrUserName} {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	policiesAttr, ok := resp.Schema.Attributes["attached_policies"].(schema.ListAttribute)
	if !ok {
		t.Fatalf("expected attached_policies ListAttribute, got %T", resp.Schema.Attributes["attached_policies"])
	}
	if !policiesAttr.IsComputed() {
		t.Fatal("expected attached_policies Computed")
	}

	userNameAttr, ok := resp.Schema.Attributes[names.AttrUserName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected user_name StringAttribute, got %T", resp.Schema.Attributes[names.AttrUserName])
	}
	if !userNameAttr.IsRequired() {
		t.Fatal("expected user_name Required")
	}
	if len(userNameAttr.Validators) == 0 {
		t.Fatal("expected user_name Validators")
	}
}

// TestUnitIAMUserPolicyAttachmentsDataSource_nameValidation verifies the Framework Validators of the user_name input
// (covers the former TC-IAM-DUPA-04 User Validation Matrix):
// valid special characters and the upper length bound of 64 pass; empty values, 65 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMUserPolicyAttachmentsDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewUserPolicyAttachmentsDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}

	userNameAttr, ok := schemaResp.Schema.Attributes[names.AttrUserName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected user_name StringAttribute, got %T", schemaResp.Schema.Attributes[names.AttrUserName])
	}
	if len(userNameAttr.Validators) == 0 {
		t.Fatal("expected user_name Validators")
	}

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "valid_special_chars", value: "User_-,.+@=123", wantError: false},
		{name: "valid_special_chars_alt", value: "test-1+2_.@hello", wantError: false},
		{name: "valid_max_length", value: strings.Repeat("a", 64), wantError: false},
		{name: "empty", value: "", wantError: true},
		{name: "too_long", value: strings.Repeat("a", 65), wantError: true},
		{name: "invalid_space", value: "t est", wantError: true},
		{name: "invalid_exclamation", value: "invalid user!", wantError: true},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var diags validator.StringResponse
			for _, v := range userNameAttr.Validators {
				var resp validator.StringResponse
				v.ValidateString(ctx, validator.StringRequest{
					ConfigValue: types.StringValue(tc.value),
				}, &resp)
				diags.Diagnostics.Append(resp.Diagnostics...)
			}

			hasError := diags.Diagnostics.HasError()
			if tc.wantError && !hasError {
				t.Fatalf("expected validation error for user_name=%q, got none", tc.value)
			}
			if !tc.wantError && hasError {
				t.Fatalf("expected no validation error for user_name=%q, got: %v", tc.value, diags.Diagnostics)
			}
		})
	}
}

// TestUnitIAMUserPolicyAttachmentsDataSource_flattenUserAttachedPolicies verifies the flatten mapping of
// description/policy_arn/policy_name/scope (including OOS metadata); when metadata is missing, description/scope fall back
// to empty strings (e.g. AWS managed policies have no OOS extension).
func TestUnitIAMUserPolicyAttachmentsDataSource_flattenUserAttachedPolicies(t *testing.T) {
	t.Parallel()

	policyARN := "arn:ctyun:iam::10qepa9i1d4wj:policy/example-user-attachment-policy"
	policies := []awstypes.AttachedPolicy{
		{
			PolicyArn:  aws.String(policyARN),
			PolicyName: aws.String("example-user-attachment-policy"),
		},
	}

	t.Run("with_metadata", func(t *testing.T) {
		output := tfiam.FlattenUserAttachedPolicies(policies, map[string]tfiam.OOSAttachedPolicyMetadata{
			policyARN: {
				Description: "Example policy attached to a user",
				Scope:       "Local",
			},
		})

		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		if got, want := output[0].PolicyArn.ValueString(), policyARN; got != want {
			t.Fatalf("policy_arn = %q, want %q", got, want)
		}
		if got, want := output[0].PolicyName.ValueString(), "example-user-attachment-policy"; got != want {
			t.Fatalf("policy_name = %q, want %q", got, want)
		}
		if got, want := output[0].Description.ValueString(), "Example policy attached to a user"; got != want {
			t.Fatalf("description = %q, want %q", got, want)
		}
		if got, want := output[0].Scope.ValueString(), "Local"; got != want {
			t.Fatalf("scope = %q, want %q", got, want)
		}
	})

	t.Run("missing_metadata", func(t *testing.T) {
		output := tfiam.FlattenUserAttachedPolicies(policies, map[string]tfiam.OOSAttachedPolicyMetadata{})

		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		if got, want := output[0].PolicyArn.ValueString(), policyARN; got != want {
			t.Fatalf("policy_arn = %q, want %q", got, want)
		}
		if got, want := output[0].PolicyName.ValueString(), "example-user-attachment-policy"; got != want {
			t.Fatalf("policy_name = %q, want %q", got, want)
		}
		if got, want := output[0].Description.ValueString(), ""; got != want {
			t.Fatalf("description = %q, want empty", got)
		}
		if got, want := output[0].Scope.ValueString(), ""; got != want {
			t.Fatalf("scope = %q, want empty", got)
		}
	})
}

// TC-IAM-DUPA-01-Basic Single Attachment
// verifies reading by user_name after creating a user and attaching 1 policy: the DS state matches the cloud
// ListAttachedUserPolicies result (including OOS metadata description/scope); description/scope/policy_arn are also
// cross-verified against the configuration-side expected values (bypassing FindUserPolicyAttachments to avoid
// self-referencing the DS Read); CheckDestroy at the end.
func TestAccIAMUserPolicyAttachmentsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_user_policy_attachments.test"
	policyResourceName := "ctyun_iam_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.0.policy_name", rName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.0.description", "test policy"),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.0.scope", "Local"),
					resource.TestCheckResourceAttrPair(dataSourceName, "attached_policies.0.policy_arn", policyResourceName, names.AttrARN),
				),
			},
		},
	})
}

// TC-IAM-DUPA-02-Empty Attachment List
// verifies reading a user that exists but has no policy attached: attached_policies is empty, id/user_name equal the user
// name, and the result matches the cloud ListAttachedUserPolicies; CheckDestroy at the end.
func TestAccIAMUserPolicyAttachmentsDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_user_policy_attachments.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_empty(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, rName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DUPA-03-Multiple Attachments
// verifies reading after attaching multiple policies to the same user: attached_policies matches the cloud
// ListAttachedUserPolicies set exactly; description/scope/policy_arn are also cross-verified by policy_name against the
// configuration-side expected values (bypassing FindUserPolicyAttachments); CheckDestroy at the end.
func TestAccIAMUserPolicyAttachmentsDataSource_multiple(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_user_policy_attachments.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_multiple(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "2"),
					testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test1", rName+"-1", "test policy 1", "Local"),
					testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test2", rName+"-2", "test policy 2", "Local"),
				),
			},
		},
	})
}

// TC-IAM-DUPA-05-Missing User Negative verifies that reading a non-existent user_name returns a clear NotFound-style error,
// and that no data.ctyun_iam_user_policy_attachments remains in state (no half-written result).
func TestAccIAMUserPolicyAttachmentsDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentsDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccUserPolicyAttachmentsDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserPolicyAttachmentsDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DUPA-05-Other Account Negative verifies that reading a user that exists under another account cannot be found by
// name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_user_policy_attachments left in state.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the user with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMUserPolicyAttachmentsDataSource_otherAccount(t *testing.T) {
	crossAccountUser := testAccCrossAccountUserName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentsDataSourceConfig_otherAccountUser(crossAccountUser),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccUserPolicyAttachmentsDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserPolicyAttachmentsDataSourceAbsentFromState(),
			},
		},
	})
}

// The ListAttachedUserPolicies default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input +
// Paginator and cannot change MaxItems, so this case creates more attachments than the default page size to force the real Read through multi-page aggregation.
const testAccIAMListAttachedUserPoliciesDefaultMaxItems = 100

// TC-IAM-DUPA-06-Pagination verifies that when the number of policies attached to a user exceeds the
// ListAttachedUserPolicies default MaxItems, the data source Read (empty ListAttachedUserPoliciesInput → default pagination)
// still aggregates and returns all attached_policies, matching the cloud.
// Creating 101 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for
// PoliciesPerAccount with other pagination/Limit ACCs.
func TestAccIAMUserPolicyAttachmentsDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyCount := testAccIAMListAttachedUserPoliciesDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_user_policy_attachments.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckPoliciesPaginationQuota(ctx, t, policyCount)
			testAccPreCheckAttachedPoliciesPerUserQuota(ctx, t, policyCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_pagination(rName, policyCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserPolicyAttachmentsListDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", strconv.Itoa(policyCount)),
				),
			},
		},
	})
}

// TC-IAM-DUPA-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud ListAttachedUserPolicies result (including description/scope/policy_arn); the second apply shows no diff
// (PreApply / PostApplyPostRefresh). The configuration reuses the multiple case. CheckDestroy at the end.
func TestAccIAMUserPolicyAttachmentsDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_user_policy_attachments.test"

	checks := resource.ComposeTestCheckFunc(
		testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "2"),
		testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test1", rName+"-1", "test policy 1", "Local"),
		testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test2", rName+"-2", "test policy 2", "Local"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_multiple(rName),
				Check:  checks,
			},
			{
				Config: testAccUserPolicyAttachmentsDataSourceConfig_multiple(rName),
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

// testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud checks that the DS state matches the cloud
// ListAttachedUserPolicies result (including description/scope from the OOS middleware) without relying on managed resource
// state: it compares id/user_name, plus the attached_policies set field by field (policy_arn/policy_name/description/scope).
func testAccCheckUserPolicyAttachmentsDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		userName := rs.Primary.Attributes[names.AttrUserName]
		if userName == "" {
			return fmt.Errorf("bad state user_name: empty")
		}
		if rs.Primary.ID != userName {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, userName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudPolicies, cloudMetadata, err := tfiam.FindUserPolicyAttachments(ctx, conn, &iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("listing attached policies for user (%s): %w", userName, err)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["attached_policies.#"])
		if err != nil {
			return fmt.Errorf("parsing attached_policies.#: %w", err)
		}
		if stateCount != len(cloudPolicies) {
			return fmt.Errorf("attached_policies count mismatch: state=%d listAttachedUserPolicies=%d", stateCount, len(cloudPolicies))
		}

		type cloudAttached struct {
			policy   awstypes.AttachedPolicy
			metadata tfiam.OOSAttachedPolicyMetadata
		}
		cloudByARN := make(map[string]cloudAttached, len(cloudPolicies))
		for _, policy := range cloudPolicies {
			arn := aws.ToString(policy.PolicyArn)
			cloudByARN[arn] = cloudAttached{
				policy:   policy,
				metadata: cloudMetadata[arn],
			}
		}

		seenARN := make(map[string]struct{}, stateCount)
		for i := 0; i < stateCount; i++ {
			prefix := fmt.Sprintf("attached_policies.%d.", i)
			stateARN := attrs[prefix+"policy_arn"]
			if stateARN == "" {
				return fmt.Errorf("empty attached_policies.%d.policy_arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate attached_policies.%d.policy_arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("attached_policies.%d.policy_arn %q not found in ListAttachedUserPolicies", i, stateARN)
			}

			if got, want := attrs[prefix+"policy_name"], aws.ToString(cloud.policy.PolicyName); got != want {
				return fmt.Errorf("bad attached_policies.%d.policy_name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"description"], cloud.metadata.Description; got != want {
				return fmt.Errorf("bad attached_policies.%d.description: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"scope"], cloud.metadata.Scope; got != want {
				return fmt.Errorf("bad attached_policies.%d.scope: got %s, want %s", i, got, want)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("attached_policies ARN set size mismatch after field checks: state=%d listAttachedUserPolicies=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

func testAccCheckUserPolicyAttachmentsDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_user_policy_attachments.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q user_name=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrUserName],
			)
		}
		return nil
	}
}

// testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs locates an entry by policy_name in the DS state and
// cross-verifies it against the configuration-side expected values (description/scope) and the managed resource ARN,
// bypassing FindUserPolicyAttachments.
func testAccCheckUserPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, policyResourceName, policyName, description, scope string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}
		wantARN := policyRS.Primary.Attributes[names.AttrARN]
		if wantARN == "" {
			return fmt.Errorf("empty %s arn", policyResourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["attached_policies.#"])
		if err != nil {
			return fmt.Errorf("parsing attached_policies.#: %w", err)
		}
		for i := 0; i < count; i++ {
			prefix := fmt.Sprintf("attached_policies.%d.", i)
			if ds.Primary.Attributes[prefix+"policy_name"] != policyName {
				continue
			}
			if got := ds.Primary.Attributes[prefix+"description"]; got != description {
				return fmt.Errorf("bad %sdescription: got %q, want %q", prefix, got, description)
			}
			if got := ds.Primary.Attributes[prefix+"scope"]; got != scope {
				return fmt.Errorf("bad %sscope: got %q, want %q", prefix, got, scope)
			}
			if got := ds.Primary.Attributes[prefix+"policy_arn"]; got != wantARN {
				return fmt.Errorf("bad %spolicy_arn: got %q, want %q", prefix, got, wantARN)
			}
			return nil
		}
		return fmt.Errorf("policy_name %q not found in %s attached_policies", policyName, dataSourceName)
	}
}

func testAccUserPolicyAttachmentsDataSourceConfig_Base(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}
`, rName)
}

func testAccUserPolicyAttachmentsDataSourceConfig_basic(rName string) string {
	return acctest.ConfigCompose(testAccUserPolicyAttachmentsDataSourceConfig_Base(rName),
		fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "test policy"

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
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_user_policy_attachments" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`, rName))
}

func testAccUserPolicyAttachmentsDataSourceConfig_empty(rName string) string {
	return acctest.ConfigCompose(testAccUserPolicyAttachmentsDataSourceConfig_Base(rName),
		`
data "ctyun_iam_user_policy_attachments" "test" {
  user_name = ctyun_iam_user.test.name
}
`)
}

func testAccUserPolicyAttachmentsDataSourceConfig_multiple(rName string) string {
	return acctest.ConfigCompose(testAccUserPolicyAttachmentsDataSourceConfig_Base(rName),
		fmt.Sprintf(`
resource "ctyun_iam_policy" "test1" {
  name        = "%[1]s-1"
  description = "test policy 1"

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
}

resource "ctyun_iam_policy" "test2" {
  name        = "%[1]s-2"
  description = "test policy 2"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "s3:Get*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}

resource "ctyun_iam_user_policy_attachment" "test2" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test2.arn
}

data "ctyun_iam_user_policy_attachments" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [
    ctyun_iam_user_policy_attachment.test1,
    ctyun_iam_user_policy_attachment.test2
  ]
}
`, rName))
}

func testAccUserPolicyAttachmentsDataSourceConfig_notFound(userName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_user_policy_attachments" "test" {
  user_name = %[1]q
}
`, userName)
}

func testAccUserPolicyAttachmentsDataSourceConfig_otherAccountUser(userName string) string {
	return fmt.Sprintf(`
# The cross-account user is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_user_policy_attachments" "test" {
  user_name = %[1]q
}
`, userName)
}

func testAccUserPolicyAttachmentsDataSourceConfig_pagination(rName string, policyCount int) string {
	return acctest.ConfigCompose(testAccUserPolicyAttachmentsDataSourceConfig_Base(rName),
		fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  count       = %[2]d
  name        = "%[1]s-${count.index}"
  description = "pagination-${count.index}"

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
}

resource "ctyun_iam_user_policy_attachment" "test" {
  count      = %[2]d
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test[count.index].arn
}

data "ctyun_iam_user_policy_attachments" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`, rName, policyCount))
}

func testAccUserPolicyAttachmentsDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed user_policy_attachments data source read left no residual state
`
}

// testAccPreCheckAttachedPoliciesPerUserQuota confirms that AttachedPoliciesPerUserQuota is sufficient for a single user to attach need policies.
func testAccPreCheckAttachedPoliciesPerUserQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	quota := testAccIAMAttachedPoliciesPerUserQuota(ctx, t)
	if quota < need {
		t.Skipf("insufficient AttachedPoliciesPerUserQuota=%d need=%d", quota, need)
	}
}

// testAccCheckUserPolicyAttachmentsListDefaultPageTruncated fetches one page (without paging further) using the same empty
// ListAttachedUserPoliciesInput as the DS Read, confirming the user has more attached policies than the default MaxItems
// with IsTruncated=true (otherwise this case has not actually exercised the DS pagination path).
func testAccCheckUserPolicyAttachmentsListDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		userName := rs.Primary.Attributes[names.AttrUserName]
		if userName == "" {
			return fmt.Errorf("bad user_name: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("ListAttachedUserPolicies(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListAttachedUserPolicies page (MaxItems default %d); first page returned %d policies — DS Read would not exercise pagination", testAccIAMListAttachedUserPoliciesDefaultMaxItems, len(page.AttachedPolicies))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}
