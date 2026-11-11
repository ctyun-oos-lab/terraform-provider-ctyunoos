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

// TestUnitIAMGroupPolicyAttachmentsDataSource_schema verifies the schema shape of the ctyun_iam_group_policy_attachments data source:
// it contains attached_policies/id/group_name; attached_policies is Computed; group_name is Required and has Validators.
func TestUnitIAMGroupPolicyAttachmentsDataSource_schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewGroupPolicyAttachmentsDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var resp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	for _, name := range []string{"attached_policies", names.AttrID, names.AttrGroupName} {
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

	groupNameAttr, ok := resp.Schema.Attributes[names.AttrGroupName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected group_name StringAttribute, got %T", resp.Schema.Attributes[names.AttrGroupName])
	}
	if !groupNameAttr.IsRequired() {
		t.Fatal("expected group_name Required")
	}
	if len(groupNameAttr.Validators) == 0 {
		t.Fatal("expected group_name Validators")
	}
}

// TestUnitIAMGroupPolicyAttachmentsDataSource_nameValidation verifies the Framework Validators of the group_name input
// (covers the former TC-IAM-DGPA-04 Group Validation Matrix):
// valid special characters and the upper length bound of 128 pass; empty values, 129 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMGroupPolicyAttachmentsDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewGroupPolicyAttachmentsDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}

	groupNameAttr, ok := schemaResp.Schema.Attributes[names.AttrGroupName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected group_name StringAttribute, got %T", schemaResp.Schema.Attributes[names.AttrGroupName])
	}
	if len(groupNameAttr.Validators) == 0 {
		t.Fatal("expected group_name Validators")
	}

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "valid_special_chars", value: "Group_-,.+@=123", wantError: false},
		{name: "valid_max_length", value: strings.Repeat("a", 128), wantError: false},
		{name: "empty", value: "", wantError: true},
		{name: "too_long", value: strings.Repeat("a", 129), wantError: true},
		{name: "invalid_space", value: "t est", wantError: true},
		{name: "invalid_exclamation", value: "group!", wantError: true},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var diags validator.StringResponse
			for _, v := range groupNameAttr.Validators {
				var resp validator.StringResponse
				v.ValidateString(ctx, validator.StringRequest{
					ConfigValue: types.StringValue(tc.value),
				}, &resp)
				diags.Diagnostics.Append(resp.Diagnostics...)
			}

			hasError := diags.Diagnostics.HasError()
			if tc.wantError && !hasError {
				t.Fatalf("expected validation error for group_name=%q, got none", tc.value)
			}
			if !tc.wantError && hasError {
				t.Fatalf("expected no validation error for group_name=%q, got: %v", tc.value, diags.Diagnostics)
			}
		})
	}
}

// TestUnitIAMGroupPolicyAttachmentsDataSource_flattenGroupAttachedPolicies verifies the flatten mapping of
// description/policy_arn/policy_name/scope (including OOS metadata); when metadata is missing, description/scope fall back
// to empty strings (e.g. AWS managed policies have no OOS extension).
func TestUnitIAMGroupPolicyAttachmentsDataSource_flattenGroupAttachedPolicies(t *testing.T) {
	t.Parallel()

	policyARN := "arn:ctyun:iam::10qepa9i1d4wj:policy/example-group-attachment-policy"
	policies := []awstypes.AttachedPolicy{
		{
			PolicyArn:  aws.String(policyARN),
			PolicyName: aws.String("example-group-attachment-policy"),
		},
	}

	t.Run("with_metadata", func(t *testing.T) {
		output := tfiam.FlattenGroupAttachedPolicies(policies, map[string]tfiam.OOSAttachedPolicyMetadata{
			policyARN: {
				Description: "Example policy attached to a group",
				Scope:       "Local",
			},
		})

		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		if got, want := output[0].PolicyArn.ValueString(), policyARN; got != want {
			t.Fatalf("policy_arn = %q, want %q", got, want)
		}
		if got, want := output[0].PolicyName.ValueString(), "example-group-attachment-policy"; got != want {
			t.Fatalf("policy_name = %q, want %q", got, want)
		}
		if got, want := output[0].Description.ValueString(), "Example policy attached to a group"; got != want {
			t.Fatalf("description = %q, want %q", got, want)
		}
		if got, want := output[0].Scope.ValueString(), "Local"; got != want {
			t.Fatalf("scope = %q, want %q", got, want)
		}
	})

	t.Run("missing_metadata", func(t *testing.T) {
		output := tfiam.FlattenGroupAttachedPolicies(policies, map[string]tfiam.OOSAttachedPolicyMetadata{})

		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		if got, want := output[0].PolicyArn.ValueString(), policyARN; got != want {
			t.Fatalf("policy_arn = %q, want %q", got, want)
		}
		if got, want := output[0].PolicyName.ValueString(), "example-group-attachment-policy"; got != want {
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

// TC-IAM-DGPA-01-Basic Single Attachment
// verifies reading by group_name after creating a group and attaching 1 policy: the DS state matches the cloud
// ListAttachedGroupPolicies result (including OOS metadata description/scope); description/scope/policy_arn are also
// cross-verified against the configuration-side expected values (bypassing FindGroupPolicyAttachments to avoid
// self-referencing the DS Read); CheckDestroy at the end.
func TestAccIAMGroupPolicyAttachmentsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group_policy_attachments.test"
	policyResourceName := "ctyun_iam_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
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

// TC-IAM-DGPA-02-Empty Attachment List
// verifies reading a group that exists but has no policy attached: attached_policies is empty, id/group_name equal the group
// name, and the result matches the cloud ListAttachedGroupPolicies; CheckDestroy at the end.
func TestAccIAMGroupPolicyAttachmentsDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group_policy_attachments.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_empty(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrGroupName, rName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DGPA-03-Multiple Attachments
// verifies reading after attaching multiple policies to the same group: attached_policies matches the cloud
// ListAttachedGroupPolicies set exactly; description/scope/policy_arn are also cross-verified by policy_name against the
// configuration-side expected values (bypassing FindGroupPolicyAttachments); CheckDestroy at the end.
func TestAccIAMGroupPolicyAttachmentsDataSource_multiple(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group_policy_attachments.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_multiple(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "2"),
					testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test1", rName+"-1", "test policy 1", "Local"),
					testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test2", rName+"-2", "test policy 2", "Local"),
				),
			},
		},
	})
}

// TC-IAM-DGPA-05-Missing Group Negative verifies that reading a non-existent group_name returns a clear NotFound-style error,
// and that no data.ctyun_iam_group_policy_attachments remains in state (no half-written result).
func TestAccIAMGroupPolicyAttachmentsDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentsDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccGroupPolicyAttachmentsDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckGroupPolicyAttachmentsDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DGPA-05-Other Account Negative verifies that reading a group that exists under another account cannot be found by
// name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_group_policy_attachments left in state.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the group with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMGroupPolicyAttachmentsDataSource_otherAccount(t *testing.T) {
	crossAccountGroup := testAccCrossAccountGroupName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentsDataSourceConfig_otherAccountGroup(crossAccountGroup),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccGroupPolicyAttachmentsDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckGroupPolicyAttachmentsDataSourceAbsentFromState(),
			},
		},
	})
}

// The ListAttachedGroupPolicies default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input +
// Paginator and cannot change MaxItems, so this case creates more attachments than the default page size to force the real Read through multi-page aggregation.
const testAccIAMListAttachedGroupPoliciesDefaultMaxItems = 100

// TC-IAM-DGPA-06-Pagination verifies that when the number of policies attached to a group exceeds the
// ListAttachedGroupPolicies default MaxItems, the data source Read (empty ListAttachedGroupPoliciesInput → default pagination)
// still aggregates and returns all attached_policies, matching the cloud.
// Creating 101 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for
// PoliciesPerAccount with other pagination/Limit ACCs.
func TestAccIAMGroupPolicyAttachmentsDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyCount := testAccIAMListAttachedGroupPoliciesDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_group_policy_attachments.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckPoliciesPaginationQuota(ctx, t, policyCount)
			testAccPreCheckAttachedPoliciesPerGroupQuota(ctx, t, policyCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_pagination(rName, policyCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupPolicyAttachmentsListDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", strconv.Itoa(policyCount)),
				),
			},
		},
	})
}

// TC-IAM-DGPA-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud ListAttachedGroupPolicies result (including description/scope/policy_arn); the second apply shows no diff
// (PreApply / PostApplyPostRefresh). The configuration reuses the multiple case. CheckDestroy at the end.
func TestAccIAMGroupPolicyAttachmentsDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group_policy_attachments.test"

	checks := resource.ComposeTestCheckFunc(
		testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, "attached_policies.#", "2"),
		testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test1", rName+"-1", "test policy 1", "Local"),
		testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, "ctyun_iam_policy.test2", rName+"-2", "test policy 2", "Local"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_multiple(rName),
				Check:  checks,
			},
			{
				Config: testAccGroupPolicyAttachmentsDataSourceConfig_multiple(rName),
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

// testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud checks that the DS state matches the cloud
// ListAttachedGroupPolicies result (including description/scope from the OOS middleware) without relying on managed resource
// state: it compares id/group_name, plus the attached_policies set field by field (policy_arn/policy_name/description/scope).
func testAccCheckGroupPolicyAttachmentsDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		groupName := rs.Primary.Attributes[names.AttrGroupName]
		if groupName == "" {
			return fmt.Errorf("bad state group_name: empty")
		}
		if rs.Primary.ID != groupName {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, groupName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudPolicies, cloudMetadata, err := tfiam.FindGroupPolicyAttachments(ctx, conn, &iam.ListAttachedGroupPoliciesInput{
			GroupName: aws.String(groupName),
		})
		if err != nil {
			return fmt.Errorf("listing attached policies for group (%s): %w", groupName, err)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["attached_policies.#"])
		if err != nil {
			return fmt.Errorf("parsing attached_policies.#: %w", err)
		}
		if stateCount != len(cloudPolicies) {
			return fmt.Errorf("attached_policies count mismatch: state=%d listAttachedGroupPolicies=%d", stateCount, len(cloudPolicies))
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
				return fmt.Errorf("attached_policies.%d.policy_arn %q not found in ListAttachedGroupPolicies", i, stateARN)
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
			return fmt.Errorf("attached_policies ARN set size mismatch after field checks: state=%d listAttachedGroupPolicies=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

func testAccCheckGroupPolicyAttachmentsDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_group_policy_attachments.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q group_name=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrGroupName],
			)
		}
		return nil
	}
}

// testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs locates an entry by policy_name in the DS state and
// cross-verifies it against the configuration-side expected values (description/scope) and the managed resource ARN,
// bypassing FindGroupPolicyAttachments.
func testAccCheckGroupPolicyAttachmentsDataSourcePolicyAttrs(dataSourceName, policyResourceName, policyName, description, scope string) resource.TestCheckFunc {
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

func testAccGroupPolicyAttachmentsDataSourceConfig_base(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}
`, rName)
}

func testAccGroupPolicyAttachmentsDataSourceConfig_basic(rName string) string {
	return acctest.ConfigCompose(testAccGroupPolicyAttachmentsDataSourceConfig_base(rName),
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
        "iam:ChangePassword"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_group_policy_attachments" "test" {
  group_name = ctyun_iam_group.test.name

  depends_on = [ctyun_iam_group_policy_attachment.test]
}
`, rName))
}

func testAccGroupPolicyAttachmentsDataSourceConfig_empty(rName string) string {
	return acctest.ConfigCompose(testAccGroupPolicyAttachmentsDataSourceConfig_base(rName),
		`
data "ctyun_iam_group_policy_attachments" "test" {
  group_name = ctyun_iam_group.test.name
}
`)
}

func testAccGroupPolicyAttachmentsDataSourceConfig_multiple(rName string) string {
	return acctest.ConfigCompose(testAccGroupPolicyAttachmentsDataSourceConfig_base(rName),
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
        "iam:ChangePassword"
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
        "iam:ChangePassword"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}

resource "ctyun_iam_group_policy_attachment" "test2" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test2.arn
}

data "ctyun_iam_group_policy_attachments" "test" {
  group_name = ctyun_iam_group.test.name

  depends_on = [
    ctyun_iam_group_policy_attachment.test1,
    ctyun_iam_group_policy_attachment.test2
  ]
}
`, rName))
}

func testAccGroupPolicyAttachmentsDataSourceConfig_notFound(groupName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_group_policy_attachments" "test" {
  group_name = %[1]q
}
`, groupName)
}

func testAccGroupPolicyAttachmentsDataSourceConfig_otherAccountGroup(groupName string) string {
	return fmt.Sprintf(`
# The cross-account group is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_group_policy_attachments" "test" {
  group_name = %[1]q
}
`, groupName)
}

func testAccGroupPolicyAttachmentsDataSourceConfig_pagination(rName string, policyCount int) string {
	return acctest.ConfigCompose(testAccGroupPolicyAttachmentsDataSourceConfig_base(rName),
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
        "iam:ChangePassword"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test" {
  count      = %[2]d
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test[count.index].arn
}

data "ctyun_iam_group_policy_attachments" "test" {
  group_name = ctyun_iam_group.test.name

  depends_on = [ctyun_iam_group_policy_attachment.test]
}
`, rName, policyCount))
}

func testAccGroupPolicyAttachmentsDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed group_policy_attachments data source read left no residual state
`
}

// testAccPreCheckAttachedPoliciesPerGroupQuota confirms that AttachedPoliciesPerGroupQuota is sufficient for a single group to attach need policies.
func testAccPreCheckAttachedPoliciesPerGroupQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	quota := testAccIAMAttachedPoliciesPerGroupQuota(ctx, t)
	if quota < need {
		t.Skipf("insufficient AttachedPoliciesPerGroupQuota=%d need=%d", quota, need)
	}
}

// testAccCheckGroupPolicyAttachmentsListDefaultPageTruncated fetches one page (without paging further) using the same empty
// ListAttachedGroupPoliciesInput as the DS Read, confirming the group has more attached policies than the default MaxItems
// with IsTruncated=true (otherwise this case has not actually exercised the DS pagination path).
func testAccCheckGroupPolicyAttachmentsListDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		groupName := rs.Primary.Attributes[names.AttrGroupName]
		if groupName == "" {
			return fmt.Errorf("bad group_name: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{
			GroupName: aws.String(groupName),
		})
		if err != nil {
			return fmt.Errorf("ListAttachedGroupPolicies(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListAttachedGroupPolicies page (MaxItems default %d); first page returned %d policies — DS Read would not exercise pagination", testAccIAMListAttachedGroupPoliciesDefaultMaxItems, len(page.AttachedPolicies))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}
