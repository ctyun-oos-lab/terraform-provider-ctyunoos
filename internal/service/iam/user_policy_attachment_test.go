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

// TestUnitIAMUserPolicyAttachment_resourceUserPolicyAttachmentValidation verifies the schema boundary and illegal-character validation behavior of ctyun_iam_user_policy_attachment's user/policy_arn.
//
// Positive cases: errContains is empty, meaning the ValidateFunc must not return any error.
// Negative cases: errContains lists the fields/keywords the error message must contain (matching the Schema ValidateFunc
// wording), so we assert not merely "an error occurred" but that the error hits the correct attribute and rule.
func TestUnitIAMUserPolicyAttachment_resourceUserPolicyAttachmentValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourceUserPolicyAttachment()

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string // empty means validation is expected to pass; non-empty means every substring must appear in the error text
	}{
		// user: StringLenBetween(1, 64) + charset ^[0-9A-Za-z=,.@\-_+]+$
		{name: "user_valid_special_chars", fieldName: "user", value: "User_-,.+@=123", errContains: nil},
		{name: "user_min", fieldName: "user", value: "a", errContains: nil},
		{name: "user_max", fieldName: "user", value: strings.Repeat("a", 64), errContains: nil},
		{name: "user_empty", fieldName: "user", value: "", errContains: []string{
			"expected length of user to be in the range (1 - 64)",
		}},
		{name: "user_too_long", fieldName: "user", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of user to be in the range (1 - 64)",
		}},
		{name: "user_invalid_character", fieldName: "user", value: "invalid user!", errContains: []string{
			"invalid value for user",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "user_invalid_space", fieldName: "user", value: "t est", errContains: []string{
			"invalid value for user",
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

// TC-IAM-UPA-01-Basic Attach verifies that binding a user to a managed policy succeeds, checking the user, policy_arn, and id field values.
// TC-IAM-UPA-03-List Verification verifies that in the single-binding scenario the List count is 1 and the policy name matches.
func TestAccIAMUserPolicyAttachment_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccUserPolicyAttachmentImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-UPA-02-Switch Policies verifies switching from one policy to others: the old binding is removed and the new binding takes effect.
// TC-IAM-UPA-03-List Verification verifies that in the multi-attachment scenario the List count and policy names match.
func TestAccIAMUserPolicyAttachment_switchPolicies(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_user_policy_attachment.test1"
	resourceName2 := "ctyun_iam_user_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					resource.TestCheckResourceAttr(resourceName1, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentPolicyName(&attached1, policyName1),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName1),
				),
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attachUpdate(userName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckUserPolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName2, policyName3),
				),
			},
		},
	})
}

// TC-IAM-UPA-04-Drift Recovery verifies that after SDK DetachUserPolicy, Terraform detects the drift (non-empty plan), and re-applying the original configuration restores the binding.
func TestAccIAMUserPolicyAttachment_driftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentExternalDetach(ctx, t, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
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

// TC-IAM-UPA-04-Drift Recovery sub-scenario B: after an undeclared binding is added remotely, the declared attachment remains a Noop (no perpetual diff).
// A single resource manages only the declared user-policy pair; undeclared bindings do not enter state.
// Cleanup strategy:
// 1) ExternalDetach at the end of the step2 Check (detach before destroy);
// 2) after ExternalAttach succeeds, register a t.Cleanup idempotent detach (in case PreApply failure skips the Check, or as a fallback cleanup on midway failures).
func TestAccIAMUserPolicyAttachment_driftExtraUndeclared(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	userResourceName := "ctyun_iam_user.test"
	policyResourceName2 := "ctyun_iam_policy.test2"

	var extraPolicyARN string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attachWithExtraPolicy(userName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName1),
					testAccCheckUserPolicyAttachmentExternalAttach(ctx, t, userResourceName, policyResourceName2),
					testAccCheckUserPolicyAttachmentCapturePolicyARN(policyResourceName2, &extraPolicyARN),
					testAccCheckUserPolicyAttachmentRegisterExternalDetachCleanup(t, userName, &extraPolicyARN),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName1, policyName2),
				),
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attachWithExtraPolicy(userName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName1),
					// The declared state is still policy1; the undeclared policy2 remains in the remote list but produces no attachment resource diff.
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName1, policyName2),
					// Detach the undeclared binding before destroy (with t.Cleanup as fallback when PreApply fails).
					testAccCheckUserPolicyAttachmentExternalDetachByDeps(ctx, t, userResourceName, policyResourceName2),
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

// TC-IAM-UPA-04-Drift Recovery verifies refreshing after the binding is deleted remotely: the attachment is removed from state and a Create is planned, while the user/policy are unaffected.
func TestAccIAMUserPolicyAttachment_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	userResourceName := "ctyun_iam_user.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceUserPolicyAttachment(), resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-UPA-04-Drift Recovery / User Disappears verifies refreshing after the user is deleted remotely:
// ResourceUser Delete requires force_destroy=true to detach bindings before deletion; after refresh, the user and the attachment are removed from state and a Create is planned, while the policy is unaffected.
func TestAccIAMUserPolicyAttachment_Disappears_user(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	userResourceName := "ctyun_iam_user.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attachForceDestroy(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceUser(), userResourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(userResourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-UPA-05-Import verifies that after an attachment is pre-created via SDK, terraform import brings it under management with state identical to a normal creation.
func TestAccIAMUserPolicyAttachment_importExisting(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	userResourceName := "ctyun_iam_user.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_userAndPolicy(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExternalAttach(ctx, t, userResourceName, policyResourceName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
				),
			},
			{
				Config:             testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  testAccUserPolicyAttachmentImportStateIdFromDepsFunc(userResourceName, policyResourceName),
				ImportStateCheck:   testAccUserPolicyAttachmentImportStateCheck(userName, policyName),
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
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

// TC-IAM-UPA-06-Stability verifies that re-applying the same configuration yields a Noop plan (no perpetual diff).
func TestAccIAMUserPolicyAttachment_stability(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attach(userName, policyName),
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

// TC-IAM-UPA-06-Stability verifies that re-applying in a multi-attachment scenario yields no perpetual diff.
// TC-IAM-UPA-03-List Verification verifies that the multi-binding List count and policy names match.
func TestAccIAMUserPolicyAttachment_stabilityMultiple(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_user_policy_attachment.test1"
	resourceName2 := "ctyun_iam_user_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_attachUpdate(userName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckUserPolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName2, policyName3),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccUserPolicyAttachmentConfig_attachUpdate(userName, policyName1, policyName2, policyName3),
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

// TC-IAM-UPA-06-Stability / Duplicate Binding No Diff: after declaring the same user-policy binding twice, only one binding exists remotely and re-applying yields no perpetual diff.
func TestAccIAMUserPolicyAttachment_duplicateBinding(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_user_policy_attachment.test1"
	resourceName2 := "ctyun_iam_user_policy_attachment.test2"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_duplicateAttach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, "user", userName),
					resource.TestCheckResourceAttr(resourceName2, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", policyResourceName, names.AttrARN),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckUserPolicyAttachmentPolicyName(&attached1, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccUserPolicyAttachmentConfig_duplicateAttach(userName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName),
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

// TC-IAM-UPA-07-Input Validation verifies clear errors for missing required fields, an illegal user name, an illegal policy ARN (validated by the server), and a non-existent policy.
// Some steps create the user and then fail at the Attach/validation stage; the framework's terraform destroy handles cleanup, and CheckDestroy uses DestroyDeps to verify there is no residue.
func TestAccIAMUserPolicyAttachment_inputValidation(t *testing.T) {
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentConfig_missingUser(),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "user" is required`),
			},
			{
				Config:      testAccUserPolicyAttachmentConfig_missingPolicyARN(userName),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "policy_arn" is required`),
			},
			{
				Config:      testAccUserPolicyAttachmentConfig_invalidUser(userName),
				ExpectError: regexache.MustCompile(`(?s)invalid value for user`),
			},
			{
				Config:      testAccUserPolicyAttachmentConfig_invalidPolicyARN(userName),
				ExpectError: regexache.MustCompile(`(?s)ARN not-a-valid-policy-arn is not valid`),
			},
			{
				Config:      testAccUserPolicyAttachmentConfig_policyNotFound(userName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
		},
	})
}

// TC-IAM-UPA-08-Limit And Ownership Negative verifies that attaching another account's policy returns AccessDenied.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the policy with the second account via SDK and destroys it with t.Cleanup at the end.
// The configuration creates a user under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMUserPolicyAttachment_policyOtherAccountNegative(t *testing.T) {
	policyARN := testAccCrossAccountPolicyARN(t)
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentConfig_otherAccountPolicy(userName, policyARN),
				ExpectError: regexache.MustCompile(`(?s)(AccessDenied|StatusCode: 403)`),
			},
		},
	})
}

// TC-IAM-UPA-08-Limit And Ownership Negative verifies that attaching to a non-existent user returns 404 noSuchUser.
// The configuration creates the policy; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMUserPolicyAttachment_userNotFoundNegative(t *testing.T) {
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentConfig_userNotFound(missingUserName, policyName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchUser|StatusCode: 404|The user with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-UPA-08-Limit And Ownership Negative verifies that attaching to a user under another account returns 404 noSuchUser.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the user with the second account via SDK and destroys it with t.Cleanup at the end.
// The user cannot be found by name under the current account, behaving the same as a non-existent user.
// The configuration creates a policy under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMUserPolicyAttachment_userOtherAccountNegative(t *testing.T) {
	crossAccountUser := testAccCrossAccountUserName(t)
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPolicyAttachmentConfig_otherAccountUser(policyName, crossAccountUser),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchUser|StatusCode: 404|The user with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-UPA-08-Limit And Ownership Negative reads AttachedPoliciesPerUserQuota via GetAccountSummary,
// fills the quota first and then attaches one more, expecting LimitExceeded.
// Creating quota+1 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for PoliciesPerAccount with other parallel ACCs.
// When the remaining quota is insufficient, the test is Skipped instead of failing with 409 on CreatePolicy.
func TestAccIAMUserPolicyAttachment_attachmentLimitNegative(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	quota := testAccIAMAttachedPoliciesPerUserQuota(ctx, t)
	needPolicies := quota + 1
	testAccPreCheckPoliciesPaginationQuota(ctx, t, needPolicies)

	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
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
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				// Create quota+1 policies but attach only the first quota of them, filling the user's policy quota first.
				Config: testAccUserPolicyAttachmentConfig_attachCount(userName, policyPrefix, needPolicies, quota),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, wantNamesAtQuota...),
				),
			},
			{
				// Then attach the (quota+1)th one, expected to exceed the PoliciesPerUser quota.
				Config:      testAccUserPolicyAttachmentConfig_attachCount(userName, policyPrefix, needPolicies, needPolicies),
				ExpectError: regexache.MustCompile(`(?s)(LimitExceeded|PoliciesPerUser|attachedPoliciesPerUserQuota|StatusCode: 409)`),
			},
		},
	})
}

// TC-IAM-UPA-09-ForceNew verifies that changing policy_arn on the same ctyun_iam_user_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMUserPolicyAttachment_policyARNForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	policyResourceName1 := "ctyun_iam_policy.test1"
	policyResourceName2 := "ctyun_iam_policy.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_forceNewPolicyARN(userName, policyName1, policyName2, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName1, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName1),
				),
			},
			{
				Config: testAccUserPolicyAttachmentConfig_forceNewPolicyARN(userName, policyName1, policyName2, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName2, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName2),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName, policyName2),
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

// TC-IAM-UPA-09-ForceNew verifies that changing user on the same ctyun_iam_user_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMUserPolicyAttachment_userForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	userName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserPolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserPolicyAttachmentConfig_forceNewUser(userName1, userName2, policyName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName1),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName1, policyName),
				),
			},
			{
				Config: testAccUserPolicyAttachmentConfig_forceNewUser(userName1, userName2, policyName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserPolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, "user", userName2),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckUserPolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckUserPolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName2, policyName),
					testAccCheckUserPolicyAttachmentList(ctx, t, userName1),
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

func testAccCheckUserPolicyAttachmentDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user_policy_attachment" {
				continue
			}

			_, err := tfiam.FindAttachedUserPolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes["user"], rs.Primary.Attributes["policy_arn"])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM User Policy Attachment %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckUserPolicyAttachmentDestroyDeps checks that the attachment and any user/policy that may remain from the same configuration are all destroyed.
// Negative cases often leave the created user/policy behind after an Attach failure; checking only the attachment is not enough.
func testAccCheckUserPolicyAttachmentDestroyDeps(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckUserPolicyAttachmentDestroy(ctx, t),
		testAccCheckUserDestroy(ctx, t),
		testAccCheckPolicyDestroy(ctx, t),
	)
}

func testAccCheckUserPolicyAttachmentExists(ctx context.Context, t *testing.T, n string, v *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindAttachedUserPolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes["user"], rs.Primary.Attributes["policy_arn"])
		if err != nil {
			return err
		}

		if v != nil {
			*v = *output
		}
		return nil
	}
}

// testAccCheckUserPolicyAttachmentStateMatchesCloud checks after Exists that the state matches the cloud binding: id, policy_arn.
func testAccCheckUserPolicyAttachmentStateMatchesCloud(n string, attached *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		wantARN := aws.ToString(attached.PolicyArn)
		if got := rs.Primary.Attributes["policy_arn"]; got != wantARN {
			return fmt.Errorf("bad policy_arn: got %s, want %s", got, wantARN)
		}

		wantID := fmt.Sprintf("%s/%s", rs.Primary.Attributes["user"], rs.Primary.Attributes["policy_arn"])
		if rs.Primary.ID == "" {
			return fmt.Errorf("%s: expected id non-empty", n)
		}
		if rs.Primary.ID != wantID {
			return fmt.Errorf("%s: expected id %q, got %q", n, wantID, rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckUserPolicyAttachmentPolicyName(attached *awstypes.AttachedPolicy, wantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got := aws.ToString(attached.PolicyName)
		if got != wantName {
			return fmt.Errorf("bad policy name: got %s, want %s", got, wantName)
		}
		return nil
	}
}

func testAccCheckUserPolicyAttachmentList(ctx context.Context, t *testing.T, userName string, wantPolicyNames ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		input := &iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(userName),
		}
		output, err := tfiam.FindAttachedUserPolicies(ctx, conn, input, tfslices.PredicateTrue[awstypes.AttachedPolicy]())

		if err != nil {
			return err
		}

		if got := len(output); got != len(wantPolicyNames) {
			return fmt.Errorf("UserPolicyAttachmentList(%q) count = %v, want %v", userName, got, len(wantPolicyNames))
		}

		gotNames := make(map[string]struct{}, len(output))
		for _, p := range output {
			gotNames[aws.ToString(p.PolicyName)] = struct{}{}
		}
		for _, want := range wantPolicyNames {
			if _, ok := gotNames[want]; !ok {
				return fmt.Errorf("UserPolicyAttachmentList(%q) missing policy name %q; got %v", userName, want, mapsKeys(gotNames))
			}
		}

		return nil
	}
}

func mapsKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func testAccCheckUserPolicyAttachmentExternalAttach(ctx context.Context, t *testing.T, userResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", userResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		user := userRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{
			UserName:  aws.String(user),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally attaching IAM User Policy Attachment (%s/%s): %w", user, policyARN, err)
		}

		return nil
	}
}

func testAccCheckUserPolicyAttachmentExternalDetach(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		user := rs.Primary.Attributes["user"]
		policyARN := rs.Primary.Attributes["policy_arn"]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{
			UserName:  aws.String(user),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally detaching IAM User Policy Attachment (%s/%s): %w", user, policyARN, err)
		}

		return nil
	}
}

func testAccCheckUserPolicyAttachmentExternalDetachByDeps(ctx context.Context, t *testing.T, userResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", userResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		user := userRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		return testAccDetachUserPolicyIgnoreNotFound(ctx, t, user, policyARN)
	}
}

func testAccCheckUserPolicyAttachmentCapturePolicyARN(policyResourceName string, policyARN *string) resource.TestCheckFunc {
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

// testAccCheckUserPolicyAttachmentRegisterExternalDetachCleanup registers a t.Cleanup that idempotently detaches the undeclared binding at the end of the case.
// Used to best-effort clean remote residue when a PreApply failure prevents the step2 Check (including the detach) from running.
func testAccCheckUserPolicyAttachmentRegisterExternalDetachCleanup(t *testing.T, userName string, policyARN *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		t.Cleanup(func() {
			arn := ""
			if policyARN != nil {
				arn = *policyARN
			}
			if userName == "" || arn == "" {
				return
			}
			cleanupCtx := context.Background()
			if err := testAccDetachUserPolicyIgnoreNotFound(cleanupCtx, t, userName, arn); err != nil {
				t.Errorf("cleanup externally attached IAM User Policy (%s/%s): %s", userName, arn, err)
			}
		})
		return nil
	}
}

func testAccDetachUserPolicyIgnoreNotFound(ctx context.Context, t *testing.T, userName, policyARN string) error {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	_, err := conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{
		UserName:  aws.String(userName),
		PolicyArn: aws.String(policyARN),
	})
	if err == nil || retry.NotFound(err) {
		return nil
	}
	return fmt.Errorf("detaching IAM User Policy Attachment (%s/%s): %w", userName, policyARN, err)
}

func testAccUserPolicyAttachmentImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["user"], rs.Primary.Attributes["policy_arn"]), nil
	}
}

func testAccUserPolicyAttachmentImportStateIdFromDepsFunc(userResourceName, policyResourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", userResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", policyResourceName)
		}

		return fmt.Sprintf("%s/%s", userRS.Primary.Attributes[names.AttrName], policyRS.Primary.Attributes[names.AttrARN]), nil
	}
}

func testAccUserPolicyAttachmentImportStateCheck(userName, policyName string) resource.ImportStateCheckFunc {
	return func(is []*terraform.InstanceState) error {
		var rs *terraform.InstanceState
		for _, s := range is {
			if s.Ephemeral.Type == "ctyun_iam_user_policy_attachment" {
				rs = s
				break
			}
		}
		if rs == nil {
			return fmt.Errorf("expected ctyun_iam_user_policy_attachment in import state, got %d resources", len(is))
		}

		if got := rs.Attributes["user"]; got != userName {
			return fmt.Errorf("expected user %q, got %q", userName, got)
		}

		policyARN := rs.Attributes["policy_arn"]
		if policyARN == "" {
			return fmt.Errorf("expected policy_arn to be set")
		}
		if !strings.Contains(policyARN, ":policy/"+policyName) {
			return fmt.Errorf("expected policy_arn to reference policy %q, got %q", policyName, policyARN)
		}

		wantID := fmt.Sprintf("%s/%s", userName, policyARN)
		if rs.ID != wantID {
			return fmt.Errorf("expected id %q, got %q", wantID, rs.ID)
		}

		return nil
	}
}

func testAccUserPolicyAttachmentConfig_userAndPolicy(rName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
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
`, rName, policyName)
}

func testAccUserPolicyAttachmentConfig_attach(rName, policyName string) string {
	return acctest.ConfigCompose(
		testAccUserPolicyAttachmentConfig_userAndPolicy(rName, policyName),
		`
resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`)
}

// attachForceDestroy is the same as attach but with force_destroy enabled on the user, for Disappears_user:
// CheckSDKResourceDisappears calls ResourceUser.Delete, which requires ForceDestroy=true to detach bindings before deleting the user.
func testAccUserPolicyAttachmentConfig_attachForceDestroy(rName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, rName, policyName)
}

// testAccUserPolicyAttachmentConfig_forceNewPolicyARN keeps the same user with two policies, switching the attachment between test1/test2 to trigger policy_arn ForceNew.
func testAccUserPolicyAttachmentConfig_forceNewPolicyARN(userName, policyName1, policyName2 string, useSecondPolicy bool) string {
	policyRef := "ctyun_iam_policy.test1.arn"
	if useSecondPolicy {
		policyRef = "ctyun_iam_policy.test2.arn"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = %[4]s
}
`, userName, policyName1, policyName2, policyRef)
}

// testAccUserPolicyAttachmentConfig_forceNewUser keeps two users with the same policy, switching the attachment between the users to trigger user ForceNew.
func testAccUserPolicyAttachmentConfig_forceNewUser(userName1, userName2, policyName string, useSecondUser bool) string {
	userRef := "ctyun_iam_user.test1.name"
	if useSecondUser {
		userRef = "ctyun_iam_user.test2.name"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_user" "test1" {
  name = %[1]q
}

resource "ctyun_iam_user" "test2" {
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = %[4]s
  policy_arn = ctyun_iam_policy.test1.arn
}
`, userName1, userName2, policyName, userRef)
}

func testAccUserPolicyAttachmentConfig_attachWithExtraPolicy(userName, policyName1, policyName2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, userName, policyName1, policyName2)
}

func testAccUserPolicyAttachmentConfig_duplicateAttach(rName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}

resource "ctyun_iam_user_policy_attachment" "test2" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, rName, policyName)
}

func testAccUserPolicyAttachmentConfig_attachUpdate(rName, policyName1, policyName2, policyName3 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
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

resource "ctyun_iam_user_policy_attachment" "test1" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test2.arn
}

resource "ctyun_iam_user_policy_attachment" "test2" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test3.arn
}
`, rName, policyName1, policyName2, policyName3)
}

func testAccIAMAttachedPoliciesPerUserQuota(ctx context.Context, t *testing.T) int {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	key := string(awstypes.SummaryKeyTypeAttachedPoliciesPerUserQuota)
	quota, ok := out.SummaryMap[key]
	if !ok || quota <= 0 {
		t.Fatalf("AttachedPoliciesPerUserQuota missing or invalid in SummaryMap: %#v", out.SummaryMap)
	}

	return int(quota)
}

func testAccUserPolicyAttachmentConfig_attachCount(userName, policyPrefix string, policyCount, attachCount int) string {
	var b strings.Builder

	fmt.Fprintf(&b, `
resource "ctyun_iam_user" "test" {
  name = %[1]q
}
`, userName)

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
resource "ctyun_iam_user_policy_attachment" "a%[1]d" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.p%[1]d.arn
}
`, i)
	}

	return b.String()
}

func testAccUserPolicyAttachmentConfig_missingUser() string {
	return `
resource "ctyun_iam_user_policy_attachment" "test" {
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`
}

func testAccUserPolicyAttachmentConfig_missingPolicyARN(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user = ctyun_iam_user.test.name
}
`, userName)
}

func testAccUserPolicyAttachmentConfig_invalidUser(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = "invalid user!"
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`, userName)
}

// policy_arn has a valid length (allowed by the schema) but an invalid format; the server-side AttachUserPolicy is expected to return 400.
func testAccUserPolicyAttachmentConfig_invalidPolicyARN(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = "not-a-valid-policy-arn"
}
`, userName)
}

func testAccUserPolicyAttachmentConfig_policyNotFound(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/tf-acc-nonexistent-policy"
}
`, userName)
}

func testAccUserPolicyAttachmentConfig_otherAccountPolicy(userName, policyARN string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user = ctyun_iam_user.test.name
  # The cross-account policy is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  policy_arn = %[2]q
}
`, userName, policyARN)
}

func testAccUserPolicyAttachmentConfig_userNotFound(userName, policyName string) string {
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

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = %[1]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, userName, policyName)
}

func testAccUserPolicyAttachmentConfig_otherAccountUser(policyName, userName string) string {
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

resource "ctyun_iam_user_policy_attachment" "test" {
  # The cross-account user is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  user       = %[2]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, policyName, userName)
}
