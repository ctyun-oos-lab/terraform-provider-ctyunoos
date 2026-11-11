// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMPolicy_resourcePolicyValidation verifies the schema boundary and illegal-character validation behavior of
// ctyun_iam_policy's name/name_prefix/description/policy.
//
// Positive cases: errContains is empty, meaning the ValidateFunc must not return any error.
// Negative cases: errContains lists the fields/keywords the error message must contain (matching the Schema ValidateFunc
// wording), so we assert not merely "an error occurred" but that the error hits the correct attribute and rule.
//
// The name-related cases cover the schema validation scenarios of the former TestAccIAMPolicy_nameValidation:
// name_too_long (129), name_invalid_character (illegal character), name_max (exactly 128, valid).
func TestUnitIAMPolicy_resourcePolicyValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourcePolicy()

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string // empty means validation is expected to pass; non-empty means every substring must appear in the error text
	}{
		// description: StringLenBetween(0, 1000)
		{name: "description_empty", fieldName: names.AttrDescription, value: "", errContains: nil},
		{name: "description_max", fieldName: names.AttrDescription, value: strings.Repeat("d", 1000), errContains: nil},
		{name: "description_too_long", fieldName: names.AttrDescription, value: strings.Repeat("d", 1001), errContains: []string{
			"description",
			"expected length of description to be in the range (0 - 1000)",
		}},

		// name: validResourceName(128) = length 1..128 + character set [\w+=,.@-] (\w includes underscore)
		// The following covers the former ACC nameValidation: over-length fails, illegal character fails, the 128 boundary passes (fixed length, no RandomWithPrefix concatenation overflow)
		{name: "name_valid_special_chars", fieldName: names.AttrName, value: "Policy_-,.+@=123", errContains: nil},
		{name: "name_with_underscore", fieldName: names.AttrName, value: "my_policy_name", errContains: nil},
		{name: "name_min", fieldName: names.AttrName, value: "a", errContains: nil},
		{name: "name_max", fieldName: names.AttrName, value: strings.Repeat("a", 128), errContains: nil},
		{name: "name_empty", fieldName: names.AttrName, value: "", errContains: []string{
			"expected length of name to be in the range (1 - 128)",
		}},
		{name: "name_too_long", fieldName: names.AttrName, value: strings.Repeat("a", 129), errContains: []string{
			"expected length of name to be in the range (1 - 128)",
		}},
		{name: "name_invalid_character", fieldName: names.AttrName, value: "invalid policy!", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},
		{name: "name_invalid_space", fieldName: names.AttrName, value: "t est", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},

		// name_prefix: validResourceName(128 - UniqueIDSuffixLength); over-length also reports length
		{name: "name_prefix_valid", fieldName: names.AttrNamePrefix, value: "PolicyPrefix_-,.+@=123", errContains: nil},
		{name: "name_prefix_too_long", fieldName: names.AttrNamePrefix, value: strings.Repeat("a", 128), errContains: []string{
			"expected length of name_prefix to be in the range",
		}},

		// policy: StringLenBetween(1, 131072) + ValidIAMPolicyJSON
		{name: "policy_valid", fieldName: names.AttrPolicy, value: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:List*","Resource":"*"}]}`, errContains: nil},
		{name: "policy_empty", fieldName: names.AttrPolicy, value: "", errContains: []string{
			"expected length of policy to be in the range (1 - 131072)",
			`"policy" is an empty string`,
		}},
		{name: "policy_too_long", fieldName: names.AttrPolicy, value: fmt.Sprintf(`{"Statement":"%s"}`, strings.Repeat("a", 131072)), errContains: []string{
			"expected length of policy to be in the range (1 - 131072)",
		}},
		{name: "policy_invalid_json", fieldName: names.AttrPolicy, value: "not-json", errContains: []string{
			`"policy"`,
			"invalid JSON policy",
			"not a JSON object",
		}},
		// ValidIAMPolicyJSON: TrimSpace runs first, so pure whitespace falls into empty; the "leading space characters" branch is unreachable after TrimSpace (the heredoc scenario relies on trimming first).
		{name: "policy_whitespace_only", fieldName: names.AttrPolicy, value: " \t\r\n", errContains: []string{
			`"policy" is an empty string`,
		}},
		// Leading whitespace + a valid object: should pass after TrimSpace (a common heredoc shape)
		{name: "policy_leading_whitespace_valid_object", fieldName: names.AttrPolicy, value: "\n\t{\"Version\":\"2012-10-17\",\"Statement\":[]}\n", errContains: nil},
		{name: "policy_json_string", fieldName: names.AttrPolicy, value: `"blub"`, errContains: []string{
			`"policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
		}},
		{name: "policy_json_string_filename", fieldName: names.AttrPolicy, value: `"../some-filename.json"`, errContains: []string{
			`"policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
			"JSON-encoded filename",
		}},
		{name: "policy_json_string_double_encoded", fieldName: names.AttrPolicy, value: `"{\"Version\":\"...\"}"`, errContains: []string{
			`"policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
			"double-encoded",
		}},
		{name: "policy_json_array", fieldName: names.AttrPolicy, value: `[{}]`, errContains: []string{
			`"policy"`,
			"contains a JSON array, not a JSON object",
		}},
		{name: "policy_duplicate_keys", fieldName: names.AttrPolicy, value: `{"a":"foo","a":"bar"}`, errContains: []string{
			`"policy"`,
			"contains duplicate JSON keys",
			`duplicate key "a"`,
		}},
		// Starts with { but is syntactically invalid → NormalizeJsonString / json.SyntaxError
		{name: "policy_invalid_object_syntax", fieldName: names.AttrPolicy, value: `{"def":}`, errContains: []string{
			`"policy"`,
			"invalid JSON policy",
			"byte offset",
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

// TC-IAM-POLICY-01-Basic verifies the basic CRUD flow of an IAM Policy: create the policy, read key attributes, import into state, and confirm the resource is destroyed after the test.
func TestAccIAMPolicy_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_name(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrARN),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-POLICY-01-Basic verifies that a unique name is auto-generated by Terraform when neither name nor name_prefix is specified.
func TestAccIAMPolicy_nameGenerated(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_nameGenerated(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyAttributesMatchCloud(&out),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestMatchResourceAttr(resourceName, names.AttrName, regexp.MustCompile(`^terraform-`)),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrARN),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrNamePrefix},
			},
		},
	})
}

// TC-IAM-POLICY-02-Description verifies that after setting description the field reads back correctly, is updated in place, can be cleared to "", and covers import validation.
func TestAccIAMPolicy_description(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_description(rName, "description1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, "description1"),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrARN),
				),
			},
			{
				Config: testAccPolicyConfig_description(rName, "description2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, "description2"),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrARN),
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
			// description2 → "": clearing should trigger an in-place Update (not ForceNew)
			{
				Config: testAccPolicyConfig_description(rName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrID, resourceName, names.AttrARN),
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
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-POLICY-03-Name And Length Validation verifies that after creating with name_prefix, the final name satisfies the prefix rules.
func TestAccIAMPolicy_namePrefix(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_namePrefix(acctest.ResourcePrefix),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyAttributesMatchCloud(&out),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					resource.TestMatchResourceAttr(resourceName, names.AttrName, regexache.MustCompile(fmt.Sprintf("^%s", acctest.ResourcePrefix))),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-POLICY-03-Name And Length Validation verifies that setting both name and name_prefix fails the schema ConflictsWith check.
func TestAccIAMPolicy_nameConflictsWithNamePrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyConfig_nameAndNamePrefix(rName, acctest.ResourcePrefix),
				ExpectError: regexache.MustCompile(`(?s)("name": conflicts with name_prefix|"name_prefix": conflicts with name)`),
			},
		},
	})
}

// TC-IAM-POLICY-04-Equivalent Document verifies that re-applying after only adjusting whitespace/indentation/line breaks yields no perpetual diff.
func TestAccIAMPolicy_whitespace(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_whitespace(rName, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccPolicyConfig_whitespace(rName, " ", "", ""),
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
				Config: testAccPolicyConfig_whitespace(rName, " ", "\n", ""),
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
				Config: testAccPolicyConfig_whitespace(rName, " ", "\n", " "),
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
				Config: testAccPolicyConfig_whitespace(rName, " \n", "\n", "\t "),
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

// TC-IAM-POLICY-04-Equivalent Document verifies that repeated apply/import of a complex policy document yields no perpetual diff.
func TestAccIAMPolicy_diffs(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_diffs(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccPolicyConfig_diffs(rName),
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

// TC-IAM-POLICY-04-Equivalent Document verifies that a semantically equivalent document with only the JSON field order changed yields no perpetual diff on re-plan.
func TestAccIAMPolicy_equivalentDocument(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	policy1 := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:Describe*"],"Resource":"*"}]}`
	policy2 := `{"Statement":[{"Resource":"*","Action":["ec2:Describe*"],"Effect":"Allow"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_basic(rName, policy1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDocument(ctx, t, resourceName, policy1),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccPolicyConfig_basic(rName, policy2),
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

// TC-IAM-POLICY-05-Document Update verifies that after updating the policy document, the remote side and state stay in sync; it also covers the invalid JSON negative path.
func TestAccIAMPolicy_policy(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	policy1 := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`
	policy2 := `{"Statement":[{"Action":["ec2:*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				// Schema: ValidIAMPolicyJSON → the error should name the attribute "policy"
				Config:      testAccPolicyConfig_basic(rName, "not-json"),
				ExpectError: regexache.MustCompile(`(?s)"policy" contains an invalid JSON policy`),
			},
			{
				Config: testAccPolicyConfig_basic(rName, policy1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, policy1),
				),
			},
			{
				Config: testAccPolicyConfig_basic(rName, policy2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, policy2),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-POLICY-06-Required Field Negative verifies expected failures when Effect is missing, Resource is missing, or Resource and NotResource appear together.
func TestAccIAMPolicy_requiredFieldsNegative(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyConfig_basic(rName, `{"Version":"2012-10-17","Statement":[{"Action":"s3:ListAllMyBuckets","Resource":"*"}]}`),
				ExpectError: regexache.MustCompile(`(?s)(MalformedPolicyDocument|Missing required field Effect)`),
			},
			{
				Config:      testAccPolicyConfig_basic(rName, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:ListAllMyBuckets"}]}`),
				ExpectError: regexache.MustCompile(`(?s)(MalformedPolicyDocument|Missing required field Resource)`),
			},
			{
				Config:      testAccPolicyConfig_basic(rName, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:ListAllMyBuckets","Resource":"*","NotResource":"*"}]}`),
				ExpectError: regexache.MustCompile(`(?s)(MalformedPolicyDocument|already has instance of Resource)`),
			},
		},
	})
}

// TC-IAM-POLICY-07-Invalid Document Negative verifies that apply fails as expected when the policy document contains duplicate JSON keys.
func TestAccIAMPolicy_policyDuplicateKeys(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				// Schema: ValidIAMPolicyJSON duplicate key → the error should contain the attribute "policy", duplicate JSON keys, and the concrete path
				Config:      testAccPolicyConfig_policyDuplicateKeys(rName),
				ExpectError: regexache.MustCompile(`(?s)"policy" contains duplicate JSON keys:.*duplicate key "Statement\.0\.Condition\.StringEquals"`),
			},
		},
	})
}

// TC-IAM-POLICY-07-Invalid Document Negative verifies that an invalid condition structure causes the update to fail, and that the configuration can subsequently be fixed and applied again.
func TestAccIAMPolicy_malformedCondition(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText1 := `{"Statement":[{"Action":["s3:ListBucket"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`
	expectedPolicyText2 := `{"Statement":[{"Action":["s3:ListBucket"],"Condition":{"StringLike":["demo-prefix/"]},"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`
	expectedPolicyText3 := `{"Statement":[{"Action":["s3:ListBucket"],"Condition":{"StringLike":{"s3:prefix":["demo-prefix/"]}},"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_MalformedCondition_setup(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText1),
				),
			},
			{
				Config:      testAccPolicyConfig_MalformedCondition_failure(rName),
				ExpectError: regexache.MustCompile(`(?s)(MalformedPolicyDocument: Syntax errors in policy)`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					// Because this is a Plugin SDK V2-based resource, the malformed content
					// is stored in state despite the failed update.
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText2),
				),
			},
			{
				Config: testAccPolicyConfig_MalformedCondition_fix(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					testAccCheckPolicyDescription(&out, ""),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText3),
				),
			},
		},
	})
}

// TC-IAM-POLICY-07-Invalid Document Negative verifies that an invalid condition operator fails apply as expected with a clear error.
// dateGreaterThan is valid JSON, so it passes schema validation and reaches CreatePolicy; PolicyDestroy is used to prevent residue in state on the failure path.
func TestAccIAMPolicy_invalidConditionOperator(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyConfig_basic(rName, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:Get*"],"Resource":"*","Condition":{"dateGreaterThan":{"ctyun:CurrentTime":"2017-07-01T00:00:00Z"}}}]}`),
				ExpectError: regexache.MustCompile(`(?s)(MalformedPolicyDocument|Invalid Condition type)`),
			},
		},
	})
}

// TC-IAM-POLICY-08-Disappears verifies that after the policy is deleted remotely and refreshed, Terraform detects the resource is gone and produces a non-empty plan.
func TestAccIAMPolicy_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_name(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, rName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourcePolicy(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-IAM-POLICY-09-Delete In Use Negative verifies that deleting a policy attached to a user fails with DeleteConflict as expected.
// Within the same apply, the policy is created before the attachment, so attachment_count in state may still be 0;
// therefore the remote count is verified via GetPolicy first, and state is verified after a refresh in the next step.
func TestAccIAMPolicy_deleteInUseUser(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["iam:ChangePassword"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_attachedToUser(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					// Within the same apply, the policy state may not have been refreshed after the attachment; attachment_count is not compared here.
					testAccCheckPolicyDescription(&out, "A test policy"),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyAttachmentCount(ctx, t, resourceName, 1),
					testAccCheckPolicyDeleteInUse(ctx, t, resourceName),
				),
			},
			{
				Config: testAccPolicyConfig_attachedToUser(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, policyName),
				),
			},
		},
	})
}

// TC-IAM-POLICY-09-Delete In Use Negative verifies that deleting a policy attached to a group fails with DeleteConflict as expected.
func TestAccIAMPolicy_deleteInUseGroup(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["iam:ChangePassword"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_attachedToGroup(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					// Within the same apply, the policy state may not have been refreshed after the attachment; attachment_count is not compared here.
					testAccCheckPolicyDescription(&out, "A test policy"),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyAttachmentCount(ctx, t, resourceName, 1),
					testAccCheckPolicyDeleteInUse(ctx, t, resourceName),
				),
			},
			{
				Config: testAccPolicyConfig_attachedToGroup(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, policyName),
				),
			},
		},
	})
}

// TC-IAM-POLICY-09-Delete In Use Negative verifies that deleting a policy attached to a role fails with DeleteConflict as expected.
func TestAccIAMPolicy_deleteInUseRole(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["iam:ChangePassword"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig_attachedToRole(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					// Within the same apply, the policy state may not have been refreshed after the attachment; attachment_count is not compared here.
					testAccCheckPolicyDescription(&out, "A test policy"),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyAttachmentCount(ctx, t, resourceName, 1),
					testAccCheckPolicyDeleteInUse(ctx, t, resourceName),
				),
			},
			{
				Config: testAccPolicyConfig_attachedToRole(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, policyName),
				),
			},
		},
	})
}

// TC-IAM-POLICY-10-Name ForceNew verifies ForceNew replacement for name / name_prefix:
// 1) change name → Replace; 2) change name_prefix → Replace;
// 3) name → name_prefix → Replace; 4) name_prefix → name → Replace.
func TestAccIAMPolicy_nameForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var out awstypes.Policy
	var previousARN string
	name1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	prefix1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix) + "-"
	prefix2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix) + "-"
	resourceName := "ctyun_iam_policy.test"
	expectedPolicyText := `{"Statement":[{"Action":["ec2:Describe*"],"Effect":"Allow","Resource":"*"}],"Version":"2012-10-17"}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			// baseline: create with name
			{
				Config: testAccPolicyConfig_name(name1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, name1),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyCaptureARN(&out, &previousARN),
				),
			},
			// 1) Change name → Destroy + Create
			{
				Config: testAccPolicyConfig_name(name2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, name2),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyGoneByARN(ctx, t, &previousARN),
					testAccCheckPolicyCaptureARN(&out, &previousARN),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 3) name → name_prefix → Destroy + Create
			{
				Config: testAccPolicyConfig_namePrefix(prefix1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyAttributesMatchCloud(&out),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrName, prefix1),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, prefix1),
					testAccCheckPolicyGoneByARN(ctx, t, &previousARN),
					testAccCheckPolicyCaptureARN(&out, &previousARN),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 2) Change name_prefix → Destroy + Create
			{
				Config: testAccPolicyConfig_namePrefix(prefix2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyAttributesMatchCloud(&out),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrName, prefix2),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, prefix2),
					testAccCheckPolicyGoneByARN(ctx, t, &previousARN),
					testAccCheckPolicyCaptureARN(&out, &previousARN),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 4) name_prefix → name → Destroy + Create
			{
				Config: testAccPolicyConfig_name(name3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPolicyExists(ctx, t, resourceName, &out),
					testAccCheckPolicyStateMatchesCloud(&out, name3),
					testAccCheckPolicyDocument(ctx, t, resourceName, expectedPolicyText),
					testAccCheckPolicyGoneByARN(ctx, t, &previousARN),
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

func testAccCheckPolicyExists(ctx context.Context, t *testing.T, n string, v *awstypes.Policy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindPolicyByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		*v = *output
		return nil
	}
}

// testAccCheckPolicyStateMatchesCloud checks after Exists that the state matches the cloud resource:
// id(=arn), name, arn, policy_id, attachment_count.
func testAccCheckPolicyStateMatchesCloud(policy *awstypes.Policy, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.ToString(policy.PolicyName) != name {
			return fmt.Errorf("bad remote name: %s", aws.ToString(policy.PolicyName))
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_policy" {
				continue
			}

			if rs.Primary.Attributes[names.AttrName] != name {
				return fmt.Errorf("bad state name: got %s, want %s", rs.Primary.Attributes[names.AttrName], name)
			}

			wantARN := aws.ToString(policy.Arn)
			if rs.Primary.ID == "" {
				return fmt.Errorf("bad state id: empty")
			}
			if rs.Primary.ID != wantARN {
				return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, wantARN)
			}
			if got := rs.Primary.Attributes[names.AttrARN]; got != wantARN {
				return fmt.Errorf("bad arn: got %s, want %s", got, wantARN)
			}
			if got, want := rs.Primary.Attributes["policy_id"], aws.ToString(policy.PolicyId); got != want {
				return fmt.Errorf("bad policy_id: got %s, want %s", got, want)
			}
			if got, want := rs.Primary.Attributes["attachment_count"], strconv.Itoa(int(aws.ToInt32(policy.AttachmentCount))); got != want {
				return fmt.Errorf("bad attachment_count: got %s, want %s", got, want)
			}

			return nil
		}

		return fmt.Errorf("ctyun_iam_policy not found in state")
	}
}

// testAccCheckPolicyAttributesMatchCloud is used when name/name_prefix are auto-generated:
// it reads the cloud PolicyName at Check execution time, avoiding early evaluation against an empty out at Compose time.
func testAccCheckPolicyAttributesMatchCloud(policy *awstypes.Policy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return testAccCheckPolicyStateMatchesCloud(policy, aws.ToString(policy.PolicyName))(s)
	}
}

func testAccCheckPolicyDescription(policy *awstypes.Policy, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if got := aws.ToString(policy.Description); got != want {
			return fmt.Errorf("bad remote description: got %s, want %s", got, want)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_policy" {
				continue
			}

			got := rs.Primary.Attributes[names.AttrDescription]
			if got != want {
				return fmt.Errorf("bad state description: got %s, want %s", got, want)
			}
			return nil
		}

		return fmt.Errorf("ctyun_iam_policy not found in state")
	}
}

func testAccCheckPolicyDocument(ctx context.Context, t *testing.T, n string, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, policyDocument, err := tfiam.FindPolicyWithDocumentByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		policyDocument, err = url.QueryUnescape(policyDocument)
		if err != nil {
			return fmt.Errorf("parsing IAM Policy (%s) document: %w", rs.Primary.ID, err)
		}

		if !verify.PolicyStringsEquivalent(want, policyDocument) {
			return fmt.Errorf("bad remote policy document: got %s, want %s", policyDocument, want)
		}

		statePolicy := rs.Primary.Attributes[names.AttrPolicy]
		if !verify.PolicyStringsEquivalent(statePolicy, policyDocument) {
			return fmt.Errorf("bad state policy: got %s, want %s", statePolicy, policyDocument)
		}

		return nil
	}
}

// testAccCheckPolicyAttachmentCount verifies the remote AttachmentCount via GetPolicy.
// Within the same apply, the policy is written to state before the attachment, so the attachment_count at that moment cannot be relied upon.
func testAccCheckPolicyAttachmentCount(ctx context.Context, t *testing.T, n string, want int32) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := tfiam.FindPolicyByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		if got := aws.ToInt32(output.AttachmentCount); got != want {
			return fmt.Errorf("expected IAM Policy (%s) AttachmentCount %d, got %d", rs.Primary.ID, want, got)
		}

		return nil
	}
}

func testAccCheckPolicyCaptureARN(policy *awstypes.Policy, arn *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		*arn = aws.ToString(policy.Arn)
		if *arn == "" {
			return fmt.Errorf("expected policy ARN to be set")
		}
		return nil
	}
}

func testAccCheckPolicyGoneByARN(ctx context.Context, t *testing.T, arn *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if arn == nil || *arn == "" {
			return fmt.Errorf("previous policy ARN is empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindPolicyByARN(ctx, conn, *arn)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("IAM Policy %s still exists after ForceNew replace", *arn)
	}
}

func testAccCheckPolicyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_policy" {
				continue
			}

			_, err := tfiam.FindPolicyByARN(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Policy %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckPolicyDeleteInUse(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.DeletePolicy(ctx, &iam.DeletePolicyInput{
			PolicyArn: aws.String(rs.Primary.ID),
		})
		if err == nil {
			return fmt.Errorf("expected DeleteConflict when deleting attached IAM Policy (%s), got success", rs.Primary.ID)
		}
		// OOS often returns a GenericAPIError(Code=DeleteConflict), not necessarily the SDK's strongly typed DeleteConflictException.
		if !errs.IsA[*awstypes.DeleteConflictException](err) &&
			!tfawserr.ErrCodeEquals(err, (&awstypes.DeleteConflictException{}).ErrorCode()) &&
			!tfawserr.ErrCodeEquals(err, "DeleteConflict") {
			return fmt.Errorf("expected DeleteConflict when deleting attached IAM Policy (%s), got: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

func testAccPolicyConfig_description(rName, description string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  description = %[1]q
  name        = %[2]q

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
`, description, rName)
}

func testAccPolicyConfig_name(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

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
`, rName)
}

func testAccPolicyConfig_nameGenerated() string {
	return `
resource "ctyun_iam_policy" "test" {
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
`
}

func testAccPolicyConfig_whitespace(rName, ws1, ws2, ws3 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = <<EOF
%[2]s{
  "Version": "2012-10-17",%[3]s
  "Statement": [
    {
      "Action": [
        "ec2:Describe*"
      ],
      "Effect": %[3]s"Allow",
      "Resource": "*"
    }
  ]
}
EOF
}
`, rName, ws1, ws2, ws3)
}

func testAccPolicyConfig_namePrefix(namePrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name_prefix = %[1]q

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
`, namePrefix)
}

func testAccPolicyConfig_nameAndNamePrefix(name, namePrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  name_prefix = %[2]q

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
`, name, namePrefix)
}

func testAccPolicyConfig_basic(rName, policy string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name   = %[1]q
  policy = %[2]q
}
`, rName, policy)
}

func testAccPolicyConfig_diffs(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = jsonencode({
    Id      = %[1]q
    Version = "2012-10-17"
    Statement = [{
      Sid    = "60c9d11f"
      Effect = "Allow"
      Action = [
        "kms:Describe*",
        "kms:Get*",
        "kms:List*",
        "kms:CreateKey",
        "kms:DescribeKey",
        "kms:ScheduleKeyDeletion",
        "kms:TagResource",
        "kms:UntagResource",
        "s3:ListMultipartUploadParts",
        "s3:ListBucketVersions",
        "s3:ListBucketMultipartUploads",
        "s3:ListBucket",
        "s3:GetReplicationConfiguration",
        "s3:GetObjectVersionTorrent",
        "s3:GetObjectVersionTagging",
        "s3:GetObjectVersionForReplication",
        "s3:GetObjectVersionAcl",
        "s3:GetObjectVersion",
        "s3:GetObjectTorrent",
        "s3:GetObjectTagging",
        "s3:GetObjectRetention",
        "s3:GetObjectLegalHold",
        "s3:GetObjectAcl",
        "s3:GetObject",
        "s3:GetMetricsConfiguration",
        "s3:GetLifecycleConfiguration",
        "s3:GetJobTagging",
        "s3:GetInventoryConfiguration",
        "s3:GetEncryptionConfiguration",
        "s3:GetBucketWebsite",
        "s3:GetBucketVersioning",
        "s3:GetBucketTagging",
        "s3:GetBucketRequestPayment",
        "s3:GetBucketPublicAccessBlock",
        "s3:GetBucketPolicyStatus",
        "s3:GetBucketPolicy",
        "s3:GetBucketOwnershipControls",
        "s3:GetBucketObjectLockConfiguration",
        "s3:GetBucketNotification",
        "s3:GetBucketLogging",
        "s3:GetBucketLocation",
        "s3:GetBucketCORS",
        "s3:GetBucketAcl",
        "s3:GetAnalyticsConfiguration",
        "s3:GetAccessPointPolicyStatus",
        "s3:GetAccessPointPolicy",
        "s3:GetAccelerateConfiguration",
        "s3:DescribeJob",
      ]
      Resource = "*"
    }]
  })

}
`, rName)
}

func testAccPolicyConfig_policyDuplicateKeys(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:PutObject",
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "s3:prefix": ["one/", "two/"]
        },
        "StringEquals": {
          "s3:versionid": "abc123"
        }
      }
    }
  ]
}
EOF
}
`, rName)
}

func testAccPolicyConfig_MalformedCondition_setup(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Resource = "*"
        Action = [
          "s3:ListBucket",
        ]
      },
    ]
  })
}
`, rName)
}

func testAccPolicyConfig_MalformedCondition_failure(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Resource = "*"
        Action = [
          "s3:ListBucket",
        ]
        "Condition" : {
          "StringLike" : ["demo-prefix/"]
        }
      },
    ]
  })
}
`, rName)
}

func testAccPolicyConfig_MalformedCondition_fix(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name = %[1]q

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Resource = "*"
        Action = [
          "s3:ListBucket",
        ]
        "Condition" : {
          "StringLike" : {
            "s3:prefix" : ["demo-prefix/"]
          }
        }
      },
    ]
  })
}
`, rName)
}

func testAccPolicyConfig_attachedToUser(userName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test" {
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
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}
`, userName, policyName)
}

func testAccPolicyConfig_attachedToGroup(groupName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_policy" "test" {
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
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}
`, groupName, policyName)
}

func testAccPolicyConfig_attachedToRole(roleName, policyName string) string {
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

resource "ctyun_iam_policy" "test" {
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
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test.arn
}
`, roleName, policyName)
}
