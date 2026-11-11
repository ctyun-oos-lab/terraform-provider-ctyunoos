// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	tfslices "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/slices"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMGroupPolicyAttachment_resourceGroupPolicyAttachmentValidation verifies the schema boundary and illegal-character validation behavior of ctyun_iam_group_policy_attachment's group/policy_arn.
func TestUnitIAMGroupPolicyAttachment_resourceGroupPolicyAttachmentValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourceGroupPolicyAttachment()

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string
	}{
		// group: StringLenBetween(1, 64) + charset ^[0-9A-Za-z=,.@\-_+]+$
		{name: "group_valid_special_chars", fieldName: "group", value: "Group_-,.+@=123", errContains: nil},
		{name: "group_min", fieldName: "group", value: "a", errContains: nil},
		{name: "group_max", fieldName: "group", value: strings.Repeat("a", 64), errContains: nil},
		{name: "group_empty", fieldName: "group", value: "", errContains: []string{
			"expected length of group to be in the range (1 - 64)",
		}},
		{name: "group_too_long", fieldName: "group", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of group to be in the range (1 - 64)",
		}},
		{name: "group_invalid_character", fieldName: "group", value: "invalid group!", errContains: []string{
			"invalid value for group",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "group_invalid_space", fieldName: "group", value: "t est", errContains: []string{
			"invalid value for group",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},

		// policy_arn: StringLenBetween(20, 2048); the ARN format is validated by the server
		{name: "policy_arn_valid", fieldName: "policy_arn", value: "arn:ctyun:iam::32fefj64y54gc:policy/test_policy", errContains: nil},
		{name: "policy_arn_oos_valid", fieldName: "policy_arn", value: "arn:ctyun:iam::OOS:policy/IAMReadOnlyAccess", errContains: nil},
		{name: "policy_arn_too_short", fieldName: "policy_arn", value: "arn:short", errContains: []string{
			"expected length of policy_arn to be in the range (20 - 2048)",
		}},
		{name: "policy_arn_too_long", fieldName: "policy_arn", value: "arn:ctyun:iam::32fefj64y54gc:policy/" + strings.Repeat("a", 2048), errContains: []string{
			"expected length of policy_arn to be in the range (20 - 2048)",
		}},
		{name: "policy_arn_malformed_passes_schema", fieldName: "policy_arn", value: "not-an-arn-policy-value", errContains: nil},
		{name: "policy_arn_aws_style_passes_schema", fieldName: "policy_arn", value: "arn:aws:iam::123456789012:policy/test", errContains: nil},
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
					t.Fatalf("expected no validation error for %s=%q, got: %s", tc.fieldName, tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for %s=%q containing %v, got none", tc.fieldName, tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for %s missing %q\nfull errors: %s", tc.fieldName, want, errText)
				}
			}
		})
	}
}

// TC-IAM-GPA-01-Basic Attach verifies that binding a group to a managed policy succeeds, checking the group, policy_arn, and id field values.
// TC-IAM-GPA-04-List Verification verifies that in the single-binding scenario the List count is 1 and the policy name matches.
func TestAccIAMGroupPolicyAttachment_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccGroupPolicyAttachmentImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-GPA-02-Input Validation verifies clear errors for missing required fields, an illegal group name, and an illegal policy ARN (validated by the server).
// Some steps create the group and then fail at the Attach/validation stage; the framework's terraform destroy handles cleanup, and CheckDestroy uses DestroyDeps to verify there is no residue.
func TestAccIAMGroupPolicyAttachment_inputValidation(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentConfig_missingGroup(),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "group" is required`),
			},
			{
				Config:      testAccGroupPolicyAttachmentConfig_missingPolicyARN(groupName),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "policy_arn" is required`),
			},
			{
				Config:      testAccGroupPolicyAttachmentConfig_invalidGroup(groupName),
				ExpectError: regexache.MustCompile(`(?s)invalid value for group`),
			},
			{
				Config:      testAccGroupPolicyAttachmentConfig_invalidPolicyARN(groupName),
				ExpectError: regexache.MustCompile(`(?s)ARN not-a-valid-policy-arn is not valid`),
			},
		},
	})
}

// TC-IAM-GPA-03-Switch Policies verifies switching from one policy to others: the old binding is removed and the new binding takes effect.
// TC-IAM-GPA-04-List Verification verifies that in the multi-attachment scenario the List count and policy names match.
func TestAccIAMGroupPolicyAttachment_switchPolicies(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_group_policy_attachment.test1"
	resourceName2 := "ctyun_iam_group_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					resource.TestCheckResourceAttr(resourceName1, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached1, policyName1),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName1),
				),
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attachUpdate(groupName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName2, policyName3),
				),
			},
		},
	})
}

// TC-IAM-GPA-05-Limit And Ownership Negative verifies that attaching a non-existent policy returns a clear error.
// The configuration creates a group under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMGroupPolicyAttachment_policyNotFoundNegative(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentConfig_policyNotFound(groupName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
		},
	})
}

// TC-IAM-GPA-05-Limit And Ownership Negative verifies that attaching another account's policy returns AccessDenied.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the policy with the second account via SDK and destroys it with t.Cleanup at the end.
// The configuration creates a group under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMGroupPolicyAttachment_policyOtherAccountNegative(t *testing.T) {
	policyARN := testAccCrossAccountPolicyARN(t)
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentConfig_otherAccountPolicy(groupName, policyARN),
				ExpectError: regexache.MustCompile(`(?s)(AccessDenied|StatusCode: 403)`),
			},
		},
	})
}

// TC-IAM-GPA-05-Limit And Ownership Negative verifies that attaching to a non-existent group returns 404 noSuchGroup.
// The configuration creates the policy; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMGroupPolicyAttachment_groupNotFoundNegative(t *testing.T) {
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingGroupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentConfig_groupNotFound(missingGroupName, policyName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchGroup|StatusCode: 404|The group with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-GPA-05-Limit And Ownership Negative verifies that attaching to a group under another account returns 404 noSuchGroup.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the group with the second account via SDK and destroys it with t.Cleanup at the end.
// The group cannot be found by name under the current account, behaving the same as a non-existent group.
// The configuration creates a policy under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMGroupPolicyAttachment_groupOtherAccountNegative(t *testing.T) {
	crossAccountGroup := testAccCrossAccountGroupName(t)
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupPolicyAttachmentConfig_otherAccountGroup(policyName, crossAccountGroup),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchGroup|StatusCode: 404|The group with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-GPA-05-Limit And Ownership Negative reads AttachedPoliciesPerGroupQuota via GetAccountSummary,
// fills the quota first and then attaches one more, expecting LimitExceeded.
// Creating quota+1 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for PoliciesPerAccount with other parallel ACCs.
// When the remaining quota is insufficient, the test is Skipped instead of failing with 409 on CreatePolicy.
func TestAccIAMGroupPolicyAttachment_attachmentLimitNegative(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	quota := testAccIAMAttachedPoliciesPerGroupQuota(ctx, t)
	needPolicies := quota + 1
	testAccPreCheckPoliciesPaginationQuota(ctx, t, needPolicies)

	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyPrefix := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	wantNamesAtQuota := make([]string, quota)
	for i := 0; i < quota; i++ {
		wantNamesAtQuota[i] = fmt.Sprintf("%s-%d", policyPrefix, i)
	}

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckPoliciesPaginationQuota(ctx, t, needPolicies)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				// Create quota+1 policies but attach only the first quota of them, filling the group's policy quota first.
				Config: testAccGroupPolicyAttachmentConfig_attachCount(groupName, policyPrefix, needPolicies, quota),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, wantNamesAtQuota...),
				),
			},
			{
				// Then attach the (quota+1)th one, expected to exceed the PoliciesPerGroup quota.
				Config:      testAccGroupPolicyAttachmentConfig_attachCount(groupName, policyPrefix, needPolicies, needPolicies),
				ExpectError: regexache.MustCompile(`(?s)(LimitExceeded|PoliciesPerGroup|attachedPoliciesPerGroupQuota|StatusCode: 409)`),
			},
		},
	})
}

// TC-IAM-GPA-06-Drift Recovery verifies that after SDK DetachGroupPolicy, Terraform detects the drift, and re-applying the original configuration restores the binding.
func TestAccIAMGroupPolicyAttachment_driftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentExternalDetach(ctx, t, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-GPA-06-Drift Recovery sub-scenario B: after an undeclared binding is added remotely, the declared attachment remains a Noop (no perpetual diff).
// A single resource manages only the declared group-policy pair; undeclared bindings do not enter state.
// Cleanup strategy:
// 1) ExternalDetach at the end of the step2 Check (detach before destroy);
// 2) after ExternalAttach succeeds, register a t.Cleanup idempotent detach (in case PreApply failure skips the Check, or as a fallback cleanup on midway failures).
func TestAccIAMGroupPolicyAttachment_driftExtraUndeclared(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	groupResourceName := "ctyun_iam_group.test"
	policyResourceName2 := "ctyun_iam_policy.test2"

	var extraPolicyARN string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attachWithExtraPolicy(groupName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName1),
					testAccCheckGroupPolicyAttachmentExternalAttach(ctx, t, groupResourceName, policyResourceName2),
					testAccCheckGroupPolicyAttachmentCapturePolicyARN(policyResourceName2, &extraPolicyARN),
					testAccCheckGroupPolicyAttachmentRegisterExternalDetachCleanup(t, groupName, &extraPolicyARN),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName1, policyName2),
				),
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attachWithExtraPolicy(groupName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName1),
					// The declared state is still policy1; the undeclared policy2 remains in the remote list but produces no attachment resource diff.
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName1, policyName2),
					// Detach the undeclared binding before destroy (with t.Cleanup as fallback when PreApply fails).
					testAccCheckGroupPolicyAttachmentExternalDetachByDeps(ctx, t, groupResourceName, policyResourceName2),
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

// TC-IAM-GPA-06-Drift Recovery verifies refreshing after the binding is deleted remotely: the attachment is removed from state and a Create is planned, while the group/policy are unaffected.
//
// Disappears_group is deliberately not added (contrast with RPA's Disappears_role / UPA's Disappears_user):
//   - Disappears_role works because ResourceRole.Delete detaches first and then calls DeleteRole, so a single call makes
//     the role and its bindings vanish from the cloud together, after which refresh is asserted: role/attachment → Create,
//     policy → Noop.
//   - Disappears_user relies on force_destroy=true, with the server cleaning up dependencies during DeleteUser.
//   - ResourceGroup.Delete only calls DeleteGroup and does not detach policies first (deleteGroupPolicyAttachments is only
//     used for sweeping); with attachments still on the group, CheckSDKResourceDisappears(ResourceGroup) fails immediately,
//     so the subsequent plan assertions would never be reached.
//   - If the test manually detached first and then deleted the group, it would test the refresh of "group gone after manual
//     dependency cleanup", not the cascading behavior of Group.Delete, overlapping heavily with the existing disappears case
//     (deleting the attachment itself) for little gain.
//   - "Deleting a group that still has bindings" is better modeled as a negative/error path on the group resource side,
//     not as a Disappears mirror for GPA.
//
// Conclusion: there is no need to add Disappears_group for GPA; the refresh after a binding disappears is covered by this case.
func TestAccIAMGroupPolicyAttachment_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	groupResourceName := "ctyun_iam_group.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceGroupPolicyAttachment(), resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(groupResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-GPA-07-Import verifies that after an attachment is pre-created via SDK, terraform import brings it under management with state identical to a normal creation.
func TestAccIAMGroupPolicyAttachment_importExisting(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	groupResourceName := "ctyun_iam_group.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_groupAndPolicy(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExternalAttach(ctx, t, groupResourceName, policyResourceName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
				),
			},
			{
				Config:             testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  testAccGroupPolicyAttachmentImportStateIdFromDepsFunc(groupResourceName, policyResourceName),
				ImportStateCheck:   testAccGroupPolicyAttachmentImportStateCheck(groupName, policyName),
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
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

// TC-IAM-GPA-08-Stability verifies that re-applying the same configuration yields a Noop plan (no perpetual diff).
func TestAccIAMGroupPolicyAttachment_stability(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attach(groupName, policyName),
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

// TC-IAM-GPA-08-Stability verifies that re-applying in a multi-attachment scenario yields no perpetual diff.
// TC-IAM-GPA-04-List Verification verifies that the multi-binding List count and policy names match.
func TestAccIAMGroupPolicyAttachment_stabilityMultiple(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_group_policy_attachment.test1"
	resourceName2 := "ctyun_iam_group_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_attachUpdate(groupName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName2, policyName3),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_attachUpdate(groupName, policyName1, policyName2, policyName3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-GPA-08-Stability / Duplicate Binding: after declaring the same group-policy binding twice, only one binding exists remotely and re-applying yields no perpetual diff.
func TestAccIAMGroupPolicyAttachment_duplicateBinding(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_group_policy_attachment.test1"
	resourceName2 := "ctyun_iam_group_policy_attachment.test2"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_duplicateAttach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "group", groupName),
					resource.TestCheckResourceAttr(resourceName2, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", policyResourceName, names.AttrARN),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached1, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_duplicateAttach(groupName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-GPA-09-ForceNew verifies that changing policy_arn on the same ctyun_iam_group_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMGroupPolicyAttachment_policyARNForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	policyResourceName1 := "ctyun_iam_policy.test1"
	policyResourceName2 := "ctyun_iam_policy.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_forceNewPolicyARN(groupName, policyName1, policyName2, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName1, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName1),
				),
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_forceNewPolicyARN(groupName, policyName1, policyName2, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName2, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName2),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName, policyName2),
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

// TC-IAM-GPA-09-ForceNew verifies that changing group on the same ctyun_iam_group_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMGroupPolicyAttachment_groupForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	groupName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	groupName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_group_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupPolicyAttachmentConfig_forceNewGroup(groupName1, groupName2, policyName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName1),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName1, policyName),
				),
			},
			{
				Config: testAccGroupPolicyAttachmentConfig_forceNewGroup(groupName1, groupName2, policyName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "group", groupName2),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckGroupPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckGroupPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName2, policyName),
					testAccCheckGroupPolicyAttachmentList(ctx, t, groupName1),
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

func testAccCheckGroupPolicyAttachmentDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_group_policy_attachment" {
				continue
			}

			_, err := tfiam.FindAttachedGroupPolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes["group"], rs.Primary.Attributes["policy_arn"])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Group Policy Attachment %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckGroupPolicyAttachmentDestroyDeps checks that the attachment and any group/policy that may remain from the same configuration are all destroyed.
// Negative cases often leave the created group/policy behind after an Attach failure; checking only the attachment is not enough.
func testAccCheckGroupPolicyAttachmentDestroyDeps(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckGroupPolicyAttachmentDestroy(ctx, t),
		testAccCheckGroupDestroy(ctx, t),
		testAccCheckPolicyDestroy(ctx, t),
	)
}

func testAccCheckGroupPolicyAttachmentExists(ctx context.Context, t *testing.T, n string, v *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindAttachedGroupPolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes["group"], rs.Primary.Attributes["policy_arn"])
		if err != nil {
			return err
		}

		if v != nil {
			*v = *output
		}
		return nil
	}
}

// testAccCheckGroupPolicyAttachmentStateMatchesCloud checks after Exists that the state matches the cloud binding: id, policy_arn.
func testAccCheckGroupPolicyAttachmentStateMatchesCloud(n string, attached *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		wantARN := aws.ToString(attached.PolicyArn)
		if got := rs.Primary.Attributes["policy_arn"]; got != wantARN {
			return fmt.Errorf("bad policy_arn: got %s, want %s", got, wantARN)
		}

		wantID := fmt.Sprintf("%s/%s", rs.Primary.Attributes["group"], rs.Primary.Attributes["policy_arn"])
		if rs.Primary.ID == "" {
			return fmt.Errorf("%s: expected id non-empty", n)
		}
		if rs.Primary.ID != wantID {
			return fmt.Errorf("%s: expected id %q, got %q", n, wantID, rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckGroupPolicyAttachmentPolicyName(attached *awstypes.AttachedPolicy, wantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got := aws.ToString(attached.PolicyName)
		if got != wantName {
			return fmt.Errorf("bad policy name: got %s, want %s", got, wantName)
		}
		return nil
	}
}

func testAccCheckGroupPolicyAttachmentList(ctx context.Context, t *testing.T, groupName string, wantPolicyNames ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		input := &iam.ListAttachedGroupPoliciesInput{
			GroupName: aws.String(groupName),
		}
		output, err := tfiam.FindAttachedGroupPolicies(ctx, conn, input, tfslices.PredicateTrue[*awstypes.AttachedPolicy]())

		if err != nil {
			return err
		}

		if got := len(output); got != len(wantPolicyNames) {
			return fmt.Errorf("GroupPolicyAttachmentList(%q) count = %v, want %v", groupName, got, len(wantPolicyNames))
		}

		gotNames := make(map[string]struct{}, len(output))
		for _, p := range output {
			gotNames[aws.ToString(p.PolicyName)] = struct{}{}
		}
		for _, want := range wantPolicyNames {
			if _, ok := gotNames[want]; !ok {
				return fmt.Errorf("GroupPolicyAttachmentList(%q) missing policy name %q; got %v", groupName, want, mapsKeys(gotNames))
			}
		}

		return nil
	}
}

func testAccCheckGroupPolicyAttachmentExternalAttach(ctx context.Context, t *testing.T, groupResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", groupResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		group := groupRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.AttachGroupPolicy(ctx, &iam.AttachGroupPolicyInput{
			GroupName: aws.String(group),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally attaching IAM Group Policy Attachment (%s/%s): %w", group, policyARN, err)
		}

		return nil
	}
}

func testAccCheckGroupPolicyAttachmentExternalDetach(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		group := rs.Primary.Attributes["group"]
		policyARN := rs.Primary.Attributes["policy_arn"]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.DetachGroupPolicy(ctx, &iam.DetachGroupPolicyInput{
			GroupName: aws.String(group),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally detaching IAM Group Policy Attachment (%s/%s): %w", group, policyARN, err)
		}

		return nil
	}
}

func testAccCheckGroupPolicyAttachmentExternalDetachByDeps(ctx context.Context, t *testing.T, groupResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", groupResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		group := groupRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		return testAccDetachGroupPolicyIgnoreNotFound(ctx, t, group, policyARN)
	}
}

func testAccCheckGroupPolicyAttachmentCapturePolicyARN(policyResourceName string, policyARN *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}
		arn := rs.Primary.Attributes[names.AttrARN]
		if arn == "" {
			return fmt.Errorf("%s: expected arn to be set", policyResourceName)
		}
		*policyARN = arn
		return nil
	}
}

// testAccCheckGroupPolicyAttachmentRegisterExternalDetachCleanup registers a t.Cleanup that idempotently detaches the undeclared binding at the end of the case.
// Used to best-effort clean remote residue when a PreApply failure prevents the step2 Check (including the detach) from running.
func testAccCheckGroupPolicyAttachmentRegisterExternalDetachCleanup(t *testing.T, groupName string, policyARN *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		t.Cleanup(func() {
			arn := ""
			if policyARN != nil {
				arn = *policyARN
			}
			if groupName == "" || arn == "" {
				return
			}
			cleanupCtx := context.Background()
			if err := testAccDetachGroupPolicyIgnoreNotFound(cleanupCtx, t, groupName, arn); err != nil {
				t.Errorf("cleanup externally attached IAM Group Policy (%s/%s): %s", groupName, arn, err)
			}
		})
		return nil
	}
}

func testAccDetachGroupPolicyIgnoreNotFound(ctx context.Context, t *testing.T, groupName, policyARN string) error {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	_, err := conn.DetachGroupPolicy(ctx, &iam.DetachGroupPolicyInput{
		GroupName: aws.String(groupName),
		PolicyArn: aws.String(policyARN),
	})
	if err == nil || retry.NotFound(err) {
		return nil
	}
	return fmt.Errorf("detaching IAM Group Policy Attachment (%s/%s): %w", groupName, policyARN, err)
}

func testAccGroupPolicyAttachmentImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["group"], rs.Primary.Attributes["policy_arn"]), nil
	}
}

func testAccGroupPolicyAttachmentImportStateIdFromDepsFunc(groupResourceName, policyResourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", groupResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", policyResourceName)
		}

		return fmt.Sprintf("%s/%s", groupRS.Primary.Attributes[names.AttrName], policyRS.Primary.Attributes[names.AttrARN]), nil
	}
}

func testAccGroupPolicyAttachmentImportStateCheck(groupName, policyName string) resource.ImportStateCheckFunc {
	return func(is []*terraform.InstanceState) error {
		var rs *terraform.InstanceState
		for _, s := range is {
			if s.Ephemeral.Type == "ctyun_iam_group_policy_attachment" {
				rs = s
				break
			}
		}
		if rs == nil {
			return fmt.Errorf("expected ctyun_iam_group_policy_attachment in import state, got %d resources", len(is))
		}

		if got := rs.Attributes["group"]; got != groupName {
			return fmt.Errorf("expected group %q, got %q", groupName, got)
		}

		policyARN := rs.Attributes["policy_arn"]
		if policyARN == "" {
			return fmt.Errorf("expected policy_arn to be set")
		}
		if !strings.Contains(policyARN, ":policy/"+policyName) {
			return fmt.Errorf("expected policy_arn to reference policy %q, got %q", policyName, policyARN)
		}

		wantID := fmt.Sprintf("%s/%s", groupName, policyARN)
		if rs.ID != wantID {
			return fmt.Errorf("expected id %q, got %q", wantID, rs.ID)
		}

		return nil
	}
}

func testAccGroupPolicyAttachmentConfig_groupAndPolicy(groupName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test1" {
  name        = %[2]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}
`, groupName, policyName)
}

func testAccGroupPolicyAttachmentConfig_attach(groupName, policyName string) string {
	return acctest.ConfigCompose(
		testAccGroupPolicyAttachmentConfig_groupAndPolicy(groupName, policyName),
		`
resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`)
}

// testAccGroupPolicyAttachmentConfig_forceNewPolicyARN keeps the same group with two policies, switching the attachment between test1/test2 to trigger policy_arn ForceNew.
func testAccGroupPolicyAttachmentConfig_forceNewPolicyARN(groupName, policyName1, policyName2 string, useSecondPolicy bool) string {
	policyRef := "ctyun_iam_policy.test1.arn"
	if useSecondPolicy {
		policyRef = "ctyun_iam_policy.test2.arn"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test1" {
  name        = %[2]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_policy" "test2" {
  name        = %[3]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = %[4]s
}
`, groupName, policyName1, policyName2, policyRef)
}

// testAccGroupPolicyAttachmentConfig_forceNewGroup keeps two groups with the same policy, switching the attachment between the groups to trigger group ForceNew.
func testAccGroupPolicyAttachmentConfig_forceNewGroup(groupName1, groupName2, policyName string, useSecondGroup bool) string {
	groupRef := "ctyun_iam_group.test1.name"
	if useSecondGroup {
		groupRef = "ctyun_iam_group.test2.name"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_group" "test1" {
  name = %[1]q
}

resource "ctyun_iam_group" "test2" {
  name = %[2]q
}

resource "ctyun_iam_policy" "test1" {
  name        = %[3]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = %[4]s
  policy_arn = ctyun_iam_policy.test1.arn
}
`, groupName1, groupName2, policyName, groupRef)
}

func testAccGroupPolicyAttachmentConfig_attachWithExtraPolicy(groupName, policyName1, policyName2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test1" {
  name        = %[2]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_policy" "test2" {
  name        = %[3]q
  description = "An undeclared extra policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, groupName, policyName1, policyName2)
}

func testAccGroupPolicyAttachmentConfig_duplicateAttach(groupName, policyName string) string {
	return acctest.ConfigCompose(
		testAccGroupPolicyAttachmentConfig_groupAndPolicy(groupName, policyName),
		`
resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}

resource "ctyun_iam_group_policy_attachment" "test2" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`)
}

func testAccGroupPolicyAttachmentConfig_attachUpdate(groupName, policyName1, policyName2, policyName3 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test1" {
  name        = %[2]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_policy" "test2" {
  name        = %[3]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_policy" "test3" {
  name        = %[4]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test1" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test2.arn
}

resource "ctyun_iam_group_policy_attachment" "test2" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test3.arn
}
`, groupName, policyName1, policyName2, policyName3)
}

func testAccIAMAttachedPoliciesPerGroupQuota(ctx context.Context, t *testing.T) int {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	key := string(awstypes.SummaryKeyTypeAttachedPoliciesPerGroupQuota)
	quota, ok := out.SummaryMap[key]
	if !ok || quota <= 0 {
		t.Fatalf("AttachedPoliciesPerGroupQuota missing or invalid in SummaryMap: %#v", out.SummaryMap)
	}

	return int(quota)
}

func testAccGroupPolicyAttachmentConfig_attachCount(groupName, policyPrefix string, policyCount, attachCount int) string {
	var b strings.Builder

	fmt.Fprintf(&b, `
resource "ctyun_iam_group" "test" {
  name = %[1]q
}
`, groupName)

	for i := 0; i < policyCount; i++ {
		fmt.Fprintf(&b, `
resource "ctyun_iam_policy" "p%[1]d" {
  name        = "%[2]s-%[1]d"
  description = "limit test policy %[1]d"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:GetUser"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}
`, i, policyPrefix)
	}

	for i := 0; i < attachCount; i++ {
		fmt.Fprintf(&b, `
resource "ctyun_iam_group_policy_attachment" "a%[1]d" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.p%[1]d.arn
}
`, i)
	}

	return b.String()
}

func testAccGroupPolicyAttachmentConfig_missingGroup() string {
	return `
resource "ctyun_iam_group_policy_attachment" "test" {
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`
}

func testAccGroupPolicyAttachmentConfig_missingPolicyARN(groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group = ctyun_iam_group.test.name
}
`, groupName)
}

func testAccGroupPolicyAttachmentConfig_invalidGroup(groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = "invalid group!"
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`, groupName)
}

// policy_arn has a valid length (allowed by the schema) but an invalid format; the server-side AttachGroupPolicy is expected to return 400.
func testAccGroupPolicyAttachmentConfig_invalidPolicyARN(groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = "not-a-valid-policy-arn"
}
`, groupName)
}

func testAccGroupPolicyAttachmentConfig_policyNotFound(groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/tf-acc-nonexistent-policy"
}
`, groupName)
}

func testAccGroupPolicyAttachmentConfig_otherAccountPolicy(groupName, policyARN string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group = ctyun_iam_group.test.name
  # The cross-account policy is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  policy_arn = %[2]q
}
`, groupName, policyARN)
}

func testAccGroupPolicyAttachmentConfig_groupNotFound(groupName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test1" {
  name        = %[2]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = %[1]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, groupName, policyName)
}

func testAccGroupPolicyAttachmentConfig_otherAccountGroup(policyName, groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test1" {
  name        = %[1]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "test" {
  # The cross-account group is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  group      = %[2]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, policyName, groupName)
}
