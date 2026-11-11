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

// TestUnitIAMRolePolicyAttachment_resourceRolePolicyAttachmentValidation verifies the schema boundary and illegal-character validation behavior of ctyun_iam_role_policy_attachment's role/policy_arn.
func TestUnitIAMRolePolicyAttachment_resourceRolePolicyAttachmentValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourceRolePolicyAttachment()

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string
	}{
		// role: StringLenBetween(1, 64) + charset ^[\w+=,.@-]*$
		{name: "role_valid_special_chars", fieldName: names.AttrRole, value: "Role_-,.+@=123", errContains: nil},
		{name: "role_min", fieldName: names.AttrRole, value: "a", errContains: nil},
		{name: "role_max", fieldName: names.AttrRole, value: strings.Repeat("a", 64), errContains: nil},
		{name: "role_empty", fieldName: names.AttrRole, value: "", errContains: []string{
			"expected length of role to be in the range (1 - 64)",
		}},
		{name: "role_too_long", fieldName: names.AttrRole, value: strings.Repeat("a", 65), errContains: []string{
			"expected length of role to be in the range (1 - 64)",
		}},
		{name: "role_invalid_character", fieldName: names.AttrRole, value: "invalid role!", errContains: []string{
			"must match [\\w+=,.@-]",
		}},
		{name: "role_invalid_space", fieldName: names.AttrRole, value: "t est", errContains: []string{
			"must match [\\w+=,.@-]",
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

// TC-IAM-RPA-01-Basic Attach verifies that binding a role to a managed policy succeeds, checking the role, policy_arn, and id field values.
func TestAccIAMRolePolicyAttachment_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccRolePolicyAttachmentImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-RPA-02-Switch Policies verifies switching from one policy to others: the old binding is removed and the new binding takes effect.
func TestAccIAMRolePolicyAttachment_switchPolicies(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_role_policy_attachment.test1"
	resourceName2 := "ctyun_iam_role_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					resource.TestCheckResourceAttr(resourceName1, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentPolicyName(&attached1, policyName1),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName1),
				),
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attachUpdate(roleName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckRolePolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName2, policyName3),
				),
			},
		},
	})
}

// TC-IAM-RPA-03-Role Disappears verifies refreshing after the role is deleted remotely:
// ResourceRole Delete first detaches all bindings and then calls DeleteRole; after refresh, the role and the attachment are removed from state and a Create is planned, while the policy is unaffected.
func TestAccIAMRolePolicyAttachment_Disappears_role(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	iamRoleResourceName := "ctyun_iam_role.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceRole(), iamRoleResourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(iamRoleResourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-RPA-04-Import verifies that after an attachment is pre-created via SDK, terraform import brings it under management with state identical to a normal creation.
func TestAccIAMRolePolicyAttachment_importExisting(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	roleResourceName := "ctyun_iam_role.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_roleAndPolicy(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExternalAttach(ctx, t, roleResourceName, policyResourceName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
				),
			},
			{
				Config:             testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  testAccRolePolicyAttachmentImportStateIdFromDepsFunc(roleResourceName, policyResourceName),
				ImportStateCheck:   testAccRolePolicyAttachmentImportStateCheck(roleName, policyName),
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
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

// TC-IAM-RPA-05-Drift Recovery sub-scenario A: after the current binding is manually removed remotely, drift is detected and apply restores it.
func TestAccIAMRolePolicyAttachment_driftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentExternalDetach(ctx, t, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
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

// TC-IAM-RPA-05-Drift Recovery sub-scenario B: after an undeclared binding is added remotely, the declared attachment remains a Noop (no perpetual diff).
// A single resource manages only the declared role-policy pair; undeclared bindings do not enter state.
// Cleanup strategy:
// 1) ExternalDetach at the end of the step2 Check (detach before destroy);
// 2) after ExternalAttach succeeds, register a t.Cleanup idempotent detach (in case PreApply failure skips the Check, or as a fallback cleanup on midway failures).
func TestAccIAMRolePolicyAttachment_driftExtraUndeclared(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	roleResourceName := "ctyun_iam_role.test"
	policyResourceName2 := "ctyun_iam_policy.test2"

	var extraPolicyARN string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attachWithExtraPolicy(roleName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName1),
					testAccCheckRolePolicyAttachmentExternalAttach(ctx, t, roleResourceName, policyResourceName2),
					testAccCheckRolePolicyAttachmentCapturePolicyARN(policyResourceName2, &extraPolicyARN),
					testAccCheckRolePolicyAttachmentRegisterExternalDetachCleanup(t, roleName, &extraPolicyARN),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName1, policyName2),
				),
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attachWithExtraPolicy(roleName, policyName1, policyName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", "ctyun_iam_policy.test1", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName1),
					// The declared state is still policy1; the undeclared policy2 remains in the remote list but produces no attachment resource diff.
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName1, policyName2),
					// Detach the undeclared binding before destroy (with t.Cleanup as fallback when PreApply fails).
					testAccCheckRolePolicyAttachmentExternalDetachByDeps(ctx, t, roleResourceName, policyResourceName2),
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

// TC-IAM-RPA-05-Drift Recovery verifies refreshing after the binding is deleted remotely: the attachment is removed from state and a Create is planned, while the role/policy are unaffected.
func TestAccIAMRolePolicyAttachment_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	roleResourceName := "ctyun_iam_role.test"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceRolePolicyAttachment(), resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(roleResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-RPA-06-List Order Stability verifies that re-applying the same configuration yields a Noop plan (no perpetual diff).
func TestAccIAMRolePolicyAttachment_stability(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attach(roleName, policyName),
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

// TC-IAM-RPA-06-List Order Stability verifies that re-applying in a multi-attachment scenario yields no perpetual diff (insensitive to list order).
func TestAccIAMRolePolicyAttachment_stabilityMultiple(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_role_policy_attachment.test1"
	resourceName2 := "ctyun_iam_role_policy_attachment.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_attachUpdate(roleName, policyName1, policyName2, policyName3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", "ctyun_iam_policy.test2", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentPolicyName(&attached1, policyName2),
					resource.TestCheckResourceAttr(resourceName2, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", "ctyun_iam_policy.test3", names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckRolePolicyAttachmentPolicyName(&attached2, policyName3),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName2, policyName3),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccRolePolicyAttachmentConfig_attachUpdate(roleName, policyName1, policyName2, policyName3),
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

// TC-IAM-RPA-06-List Order Stability / Duplicate Binding: after declaring the same role-policy binding twice, only one binding exists remotely and re-applying yields no perpetual diff.
func TestAccIAMRolePolicyAttachment_duplicateBinding(t *testing.T) {
	ctx := acctest.Context(t)
	var attached1, attached2 awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName1 := "ctyun_iam_role_policy_attachment.test1"
	resourceName2 := "ctyun_iam_role_policy_attachment.test2"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_duplicateAttach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					resource.TestCheckResourceAttr(resourceName1, names.AttrRole, roleName),
					resource.TestCheckResourceAttr(resourceName2, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName1, "policy_arn", policyResourceName, names.AttrARN),
					resource.TestCheckResourceAttrPair(resourceName2, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckRolePolicyAttachmentPolicyName(&attached1, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName1, plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction(resourceName2, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccRolePolicyAttachmentConfig_duplicateAttach(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName2, &attached2),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName1, &attached1),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName2, &attached2),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName),
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

// TC-IAM-RPA-07-Input Validation verifies clear errors for missing required fields, an illegal role name, an illegal policy ARN (validated by the server), and a non-existent policy.
// Some steps create the role and then fail at the Attach/validation stage; the framework's terraform destroy handles cleanup, and CheckDestroy uses DestroyDeps to verify there is no residue.
func TestAccIAMRolePolicyAttachment_inputValidation(t *testing.T) {
	ctx := acctest.Context(t)
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccRolePolicyAttachmentConfig_missingRole(),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "role" is required`),
			},
			{
				Config:      testAccRolePolicyAttachmentConfig_missingPolicyARN(roleName),
				ExpectError: regexache.MustCompile(`(?s)Missing required argument|The argument "policy_arn" is required`),
			},
			{
				Config:      testAccRolePolicyAttachmentConfig_invalidRole(roleName),
				ExpectError: regexache.MustCompile(`(?s)must match|invalid value for role`),
			},
			{
				Config:      testAccRolePolicyAttachmentConfig_invalidPolicyARN(roleName),
				ExpectError: regexache.MustCompile(`(?s)ARN not-a-valid-policy-arn is not valid`),
			},
			{
				Config:      testAccRolePolicyAttachmentConfig_policyNotFound(roleName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
		},
	})
}

// TC-IAM-RPA-08-Limit And Ownership Negative verifies that attaching another account's policy returns AccessDenied.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the policy with the second account via SDK and destroys it with t.Cleanup at the end.
// The configuration creates a role under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMRolePolicyAttachment_policyOtherAccountNegative(t *testing.T) {
	policyARN := testAccCrossAccountPolicyARN(t)
	ctx := acctest.Context(t)
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccRolePolicyAttachmentConfig_otherAccountPolicy(roleName, policyARN),
				ExpectError: regexache.MustCompile(`(?s)(AccessDenied|StatusCode: 403)`),
			},
		},
	})
}

// TC-IAM-RPA-08-Limit And Ownership Negative verifies that attaching to a non-existent role returns 404 noSuchRole.
// The configuration creates the policy; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMRolePolicyAttachment_roleNotFoundNegative(t *testing.T) {
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingRoleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccRolePolicyAttachmentConfig_roleNotFound(missingRoleName, policyName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchRole|StatusCode: 404|The role with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-RPA-08-Limit And Ownership Negative verifies that attaching to a role under another account returns 404 noSuchRole.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the role with the second account via SDK and destroys it with t.Cleanup at the end.
// The role cannot be found by name under the current account, behaving the same as a non-existent role.
// The configuration creates a policy under the current account; after the Attach fails, the framework's destroy + DestroyDeps verification handles cleanup.
func TestAccIAMRolePolicyAttachment_roleOtherAccountNegative(t *testing.T) {
	crossAccountRole := testAccCrossAccountRoleName(t)
	ctx := acctest.Context(t)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccRolePolicyAttachmentConfig_otherAccountRole(policyName, crossAccountRole),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|noSuchRole|StatusCode: 404|The role with name .* cannot be found)`),
			},
		},
	})
}

// TC-IAM-RPA-08-Limit And Ownership Negative reads AttachedPoliciesPerRoleQuota via GetAccountSummary,
// fills the quota first and then attaches one more, expecting LimitExceeded.
// Creating quota+1 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for PoliciesPerAccount with other parallel ACCs.
// When the remaining quota is insufficient, the test is Skipped instead of failing with 409 on CreatePolicy.
func TestAccIAMRolePolicyAttachment_attachmentLimitNegative(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	quota := testAccIAMAttachedPoliciesPerRoleQuota(ctx, t)
	needPolicies := quota + 1
	testAccPreCheckPoliciesPaginationQuota(ctx, t, needPolicies)

	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
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
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				// Create quota+1 policies but attach only the first quota of them, filling the role's policy quota first.
				Config: testAccRolePolicyAttachmentConfig_attachCount(roleName, policyPrefix, needPolicies, quota),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, wantNamesAtQuota...),
				),
			},
			{
				// Then attach the (quota+1)th one, expected to exceed the PoliciesPerRole quota.
				Config:      testAccRolePolicyAttachmentConfig_attachCount(roleName, policyPrefix, needPolicies, needPolicies),
				ExpectError: regexache.MustCompile(`(?s)(LimitExceeded|PoliciesPerRole|attachedPoliciesPerRoleQuota|StatusCode: 409)`),
			},
		},
	})
}

// TC-IAM-RPA-09-ForceNew verifies that changing policy_arn on the same ctyun_iam_role_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMRolePolicyAttachment_policyARNForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	policyResourceName1 := "ctyun_iam_policy.test1"
	policyResourceName2 := "ctyun_iam_policy.test2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_forceNewPolicyARN(roleName, policyName1, policyName2, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName1, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName1),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName1),
				),
			},
			{
				Config: testAccRolePolicyAttachmentConfig_forceNewPolicyARN(roleName, policyName1, policyName2, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName2, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName2),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName, policyName2),
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

// TC-IAM-RPA-09-ForceNew verifies that changing role on the same ctyun_iam_role_policy_attachment resource triggers DestroyBeforeCreate.
func TestAccIAMRolePolicyAttachment_roleForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var attached awstypes.AttachedPolicy
	roleName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	roleName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role_policy_attachment.test1"
	policyResourceName := "ctyun_iam_policy.test1"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRolePolicyAttachmentDestroyDeps(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolePolicyAttachmentConfig_forceNewRole(roleName1, roleName2, policyName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName1),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName1, policyName),
				),
			},
			{
				Config: testAccRolePolicyAttachmentConfig_forceNewRole(roleName1, roleName2, policyName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRolePolicyAttachmentExists(ctx, t, resourceName, &attached),
					resource.TestCheckResourceAttr(resourceName, names.AttrRole, roleName2),
					resource.TestCheckResourceAttrPair(resourceName, "policy_arn", policyResourceName, names.AttrARN),
					testAccCheckRolePolicyAttachmentStateMatchesCloud(resourceName, &attached),
					testAccCheckRolePolicyAttachmentPolicyName(&attached, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName2, policyName),
					testAccCheckRolePolicyAttachmentList(ctx, t, roleName1),
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

func testAccCheckRolePolicyAttachmentDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_role_policy_attachment" {
				continue
			}

			_, err := tfiam.FindAttachedRolePolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes[names.AttrRole], rs.Primary.Attributes["policy_arn"])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Role Policy Attachment %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckRolePolicyAttachmentDestroyDeps checks that the attachment and any role/policy that may remain from the same configuration are all destroyed.
// Negative cases often leave the created role/policy behind after an Attach failure; checking only the attachment is not enough.
func testAccCheckRolePolicyAttachmentDestroyDeps(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckRolePolicyAttachmentDestroy(ctx, t),
		testAccCheckRoleDestroy(ctx, t),
		testAccCheckPolicyDestroy(ctx, t),
	)
}

func testAccCheckRolePolicyAttachmentExists(ctx context.Context, t *testing.T, n string, v *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindAttachedRolePolicyByTwoPartKey(ctx, conn, rs.Primary.Attributes[names.AttrRole], rs.Primary.Attributes["policy_arn"])
		if err != nil {
			return err
		}

		if v != nil {
			*v = *output
		}
		return nil
	}
}

// testAccCheckRolePolicyAttachmentStateMatchesCloud checks after Exists that the state matches the cloud binding: id, policy_arn.
func testAccCheckRolePolicyAttachmentStateMatchesCloud(n string, attached *awstypes.AttachedPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		wantARN := aws.ToString(attached.PolicyArn)
		if got := rs.Primary.Attributes["policy_arn"]; got != wantARN {
			return fmt.Errorf("bad policy_arn: got %s, want %s", got, wantARN)
		}

		wantID := fmt.Sprintf("%s/%s", rs.Primary.Attributes[names.AttrRole], rs.Primary.Attributes["policy_arn"])
		if rs.Primary.ID == "" {
			return fmt.Errorf("%s: expected id non-empty", n)
		}
		if rs.Primary.ID != wantID {
			return fmt.Errorf("%s: expected id %q, got %q", n, wantID, rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckRolePolicyAttachmentPolicyName(attached *awstypes.AttachedPolicy, wantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got := aws.ToString(attached.PolicyName)
		if got != wantName {
			return fmt.Errorf("bad policy name: got %s, want %s", got, wantName)
		}
		return nil
	}
}

func testAccCheckRolePolicyAttachmentList(ctx context.Context, t *testing.T, roleName string, wantPolicyNames ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		input := &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		}
		output, err := tfiam.FindAttachedRolePolicies(ctx, conn, input, tfslices.PredicateTrue[awstypes.AttachedPolicy]())

		if err != nil {
			return err
		}

		if got := len(output); got != len(wantPolicyNames) {
			return fmt.Errorf("RolePolicyAttachmentList(%q) count = %v, want %v", roleName, got, len(wantPolicyNames))
		}

		gotNames := make(map[string]struct{}, len(output))
		for _, p := range output {
			gotNames[aws.ToString(p.PolicyName)] = struct{}{}
		}
		for _, want := range wantPolicyNames {
			if _, ok := gotNames[want]; !ok {
				return fmt.Errorf("RolePolicyAttachmentList(%q) missing policy name %q; got %v", roleName, want, mapsKeys(gotNames))
			}
		}

		return nil
	}
}

func testAccCheckRolePolicyAttachmentExternalAttach(ctx context.Context, t *testing.T, roleResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		roleRS, ok := s.RootModule().Resources[roleResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", roleResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		role := roleRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(role),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally attaching IAM Role Policy Attachment (%s/%s): %w", role, policyARN, err)
		}

		return nil
	}
}

func testAccCheckRolePolicyAttachmentExternalDetach(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		role := rs.Primary.Attributes[names.AttrRole]
		policyARN := rs.Primary.Attributes["policy_arn"]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(role),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("externally detaching IAM Role Policy Attachment (%s/%s): %w", role, policyARN, err)
		}

		return nil
	}
}

func testAccCheckRolePolicyAttachmentExternalDetachByDeps(ctx context.Context, t *testing.T, roleResourceName, policyResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		roleRS, ok := s.RootModule().Resources[roleResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", roleResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", policyResourceName)
		}

		role := roleRS.Primary.Attributes[names.AttrName]
		policyARN := policyRS.Primary.Attributes[names.AttrARN]
		return testAccDetachRolePolicyIgnoreNotFound(ctx, t, role, policyARN)
	}
}

func testAccCheckRolePolicyAttachmentCapturePolicyARN(policyResourceName string, policyARN *string) resource.TestCheckFunc {
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

// testAccCheckRolePolicyAttachmentRegisterExternalDetachCleanup registers a t.Cleanup that idempotently detaches the undeclared binding at the end of the case.
// Used to best-effort clean remote residue when a PreApply failure prevents the step2 Check (including the detach) from running.
func testAccCheckRolePolicyAttachmentRegisterExternalDetachCleanup(t *testing.T, roleName string, policyARN *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		t.Cleanup(func() {
			arn := ""
			if policyARN != nil {
				arn = *policyARN
			}
			if roleName == "" || arn == "" {
				return
			}
			cleanupCtx := context.Background()
			if err := testAccDetachRolePolicyIgnoreNotFound(cleanupCtx, t, roleName, arn); err != nil {
				t.Errorf("cleanup externally attached IAM Role Policy (%s/%s): %s", roleName, arn, err)
			}
		})
		return nil
	}
}

func testAccDetachRolePolicyIgnoreNotFound(ctx context.Context, t *testing.T, roleName, policyARN string) error {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	_, err := conn.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyARN),
	})
	if err == nil || retry.NotFound(err) {
		return nil
	}
	return fmt.Errorf("detaching IAM Role Policy Attachment (%s/%s): %w", roleName, policyARN, err)
}

func testAccRolePolicyAttachmentImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		return fmt.Sprintf("%s/%s", rs.Primary.Attributes[names.AttrRole], rs.Primary.Attributes["policy_arn"]), nil
	}
}

func testAccRolePolicyAttachmentImportStateIdFromDepsFunc(roleResourceName, policyResourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		roleRS, ok := s.RootModule().Resources[roleResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", roleResourceName)
		}
		policyRS, ok := s.RootModule().Resources[policyResourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", policyResourceName)
		}

		return fmt.Sprintf("%s/%s", roleRS.Primary.Attributes[names.AttrName], policyRS.Primary.Attributes[names.AttrARN]), nil
	}
}

func testAccRolePolicyAttachmentImportStateCheck(roleName, policyName string) resource.ImportStateCheckFunc {
	return func(is []*terraform.InstanceState) error {
		var rs *terraform.InstanceState
		for _, s := range is {
			if s.Ephemeral.Type == "ctyun_iam_role_policy_attachment" {
				rs = s
				break
			}
		}
		if rs == nil {
			return fmt.Errorf("expected ctyun_iam_role_policy_attachment in import state, got %d resources", len(is))
		}

		if got := rs.Attributes[names.AttrRole]; got != roleName {
			return fmt.Errorf("expected role %q, got %q", roleName, got)
		}

		policyARN := rs.Attributes["policy_arn"]
		if policyARN == "" {
			return fmt.Errorf("expected policy_arn to be set")
		}
		if !strings.Contains(policyARN, ":policy/"+policyName) {
			return fmt.Errorf("expected policy_arn to reference policy %q, got %q", policyName, policyARN)
		}

		wantID := fmt.Sprintf("%s/%s", roleName, policyARN)
		if rs.ID != wantID {
			return fmt.Errorf("expected id %q, got %q", wantID, rs.ID)
		}

		return nil
	}
}

func testAccRolePolicyAttachmentConfig_roleAndPolicy(roleName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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
`, roleName, policyName)
}

func testAccRolePolicyAttachmentConfig_attach(roleName, policyName string) string {
	return acctest.ConfigCompose(
		testAccRolePolicyAttachmentConfig_roleAndPolicy(roleName, policyName),
		`
resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`)
}

// testAccRolePolicyAttachmentConfig_forceNewPolicyARN keeps the same role with two policies, switching the attachment between test1/test2 to trigger policy_arn ForceNew.
func testAccRolePolicyAttachmentConfig_forceNewPolicyARN(roleName, policyName1, policyName2 string, useSecondPolicy bool) string {
	policyRef := "ctyun_iam_policy.test1.arn"
	if useSecondPolicy {
		policyRef = "ctyun_iam_policy.test2.arn"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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

resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = ctyun_iam_role.test.name
  policy_arn = %[4]s
}
`, roleName, policyName1, policyName2, policyRef)
}

// testAccRolePolicyAttachmentConfig_forceNewRole keeps two roles with the same policy, switching the attachment between the roles to trigger role ForceNew.
func testAccRolePolicyAttachmentConfig_forceNewRole(roleName1, roleName2, policyName string, useSecondRole bool) string {
	roleRef := "ctyun_iam_role.test1.name"
	if useSecondRole {
		roleRef = "ctyun_iam_role.test2.name"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_role" "test1" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role" "test2" {
  name = %[2]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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

resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = %[4]s
  policy_arn = ctyun_iam_policy.test1.arn
}
`, roleName1, roleName2, policyName, roleRef)
}

func testAccRolePolicyAttachmentConfig_attachWithExtraPolicy(roleName, policyName1, policyName2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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

resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, roleName, policyName1, policyName2)
}

func testAccRolePolicyAttachmentConfig_duplicateAttach(roleName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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

resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}

resource "ctyun_iam_role_policy_attachment" "test2" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test1.arn
}
`, roleName, policyName)
}

func testAccRolePolicyAttachmentConfig_attachUpdate(roleName, policyName1, policyName2, policyName3 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
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

resource "ctyun_iam_role_policy_attachment" "test1" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test2.arn
}

resource "ctyun_iam_role_policy_attachment" "test2" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test3.arn
}
`, roleName, policyName1, policyName2, policyName3)
}

func testAccIAMAttachedPoliciesPerRoleQuota(ctx context.Context, t *testing.T) int {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	key := string(awstypes.SummaryKeyTypeAttachedPoliciesPerRoleQuota)
	quota, ok := out.SummaryMap[key]
	if !ok || quota <= 0 {
		t.Fatalf("AttachedPoliciesPerRoleQuota missing or invalid in SummaryMap: %#v", out.SummaryMap)
	}

	return int(quota)
}

func testAccRolePolicyAttachmentConfig_attachCount(roleName, policyPrefix string, policyCount, attachCount int) string {
	var b strings.Builder

	fmt.Fprintf(&b, `
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}
`, roleName)

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
resource "ctyun_iam_role_policy_attachment" "a%[1]d" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.p%[1]d.arn
}
`, i)
	}

	return b.String()
}

func testAccRolePolicyAttachmentConfig_missingRole() string {
	return `
resource "ctyun_iam_role_policy_attachment" "test" {
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`
}

func testAccRolePolicyAttachmentConfig_missingPolicyARN(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role = ctyun_iam_role.test.name
}
`, roleName)
}

func testAccRolePolicyAttachmentConfig_invalidRole(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = "invalid role!"
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/test_policy"
}
`, roleName)
}

// policy_arn has a valid length (allowed by the schema) but an invalid format; the server-side AttachRolePolicy is expected to return 400.
func testAccRolePolicyAttachmentConfig_invalidPolicyARN(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = "not-a-valid-policy-arn"
}
`, roleName)
}

func testAccRolePolicyAttachmentConfig_policyNotFound(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = "arn:ctyun:iam::32fefj64y54gc:policy/tf-acc-nonexistent-policy"
}
`, roleName)
}

func testAccRolePolicyAttachmentConfig_otherAccountPolicy(roleName, policyARN string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role = ctyun_iam_role.test.name
  # The cross-account policy is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  policy_arn = %[2]q
}
`, roleName, policyARN)
}

func testAccRolePolicyAttachmentConfig_roleNotFound(roleName, policyName string) string {
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

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = %[1]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, roleName, policyName)
}

func testAccRolePolicyAttachmentConfig_otherAccountRole(policyName, roleName string) string {
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

resource "ctyun_iam_role_policy_attachment" "test" {
  # The cross-account role is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
  role       = %[2]q
  policy_arn = ctyun_iam_policy.test1.arn
}
`, policyName, roleName)
}
