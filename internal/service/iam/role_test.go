// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMRole_resourceRoleValidation verifies schema boundaries and invalid-character behavior for ctyun_iam_role assume_role_policy/description/name/name_prefix.
//
// Positive cases: an empty errContains means ValidateFunc must return no errors.
// Negative cases: errContains lists fields/keywords that must appear in the error text (matching Schema ValidateFunc wording),
// avoiding a check that only verifies whether an error occurred without confirming the correct attribute and rule.
func TestUnitIAMRole_resourceRoleValidation(t *testing.T) {
	t.Parallel()

	res := tfiam.ResourceRole()

	testCases := []struct {
		name        string
		fieldName   string
		value       string
		errContains []string // Empty means validation should pass; otherwise every substring must appear in the error text.
	}{
		// assume_role_policy: StringLenBetween(1, 131072) + validRolePolicyDocumentCharacters + ValidIAMPolicyJSON
		{name: "assume_role_policy_valid", fieldName: "assume_role_policy", value: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"sts:AssumeRole","Principal":{"AWS":"*"}}]}`, errContains: nil},
		{name: "assume_role_policy_latin1", fieldName: "assume_role_policy", value: `{"Description":"caf\u00e9","Statement":{}}`, errContains: nil},
		// tab/CR/LF are explicitly allowed by validRolePolicyDocumentCharacters.
		{name: "assume_role_policy_whitespace_chars", fieldName: "assume_role_policy", value: "{\n\t\"Version\":\"2012-10-17\",\r\n\"Statement\":[]\n}", errContains: nil},
		{name: "assume_role_policy_empty", fieldName: "assume_role_policy", value: "", errContains: []string{
			"expected length of assume_role_policy to be in the range (1 - 131072)",
			`"assume_role_policy" is an empty string`,
		}},
		{name: "assume_role_policy_too_long", fieldName: "assume_role_policy", value: fmt.Sprintf(`{"Statement":"%s"}`, strings.Repeat("a", 131073)), errContains: []string{
			"expected length of assume_role_policy to be in the range (1 - 131072)",
		}},
		{name: "assume_role_policy_invalid_character", fieldName: "assume_role_policy", value: "{\"Statement\":\"\u0100\"}", errContains: []string{
			`"assume_role_policy"`,
			"must contain only printable ASCII, Basic Latin, Latin-1 Supplement, tab, newline, or carriage return characters",
		}},
		{name: "assume_role_policy_invalid_json", fieldName: "assume_role_policy", value: "not-json", errContains: []string{
			`"assume_role_policy"`,
			"invalid JSON policy",
			"not a JSON object",
		}},
		// ValidIAMPolicyJSON trims spaces first, so pure whitespace falls into empty; the "leading space characters" branch is unreachable after TrimSpace.
		{name: "assume_role_policy_whitespace_only", fieldName: "assume_role_policy", value: " \t\r\n", errContains: []string{
			`"assume_role_policy" is an empty string`,
		}},
		// Leading whitespace plus a valid object should pass after TrimSpace (a common heredoc form).
		{name: "assume_role_policy_leading_whitespace_valid_object", fieldName: "assume_role_policy", value: "\n\t{\"Version\":\"2012-10-17\",\"Statement\":[]}\n", errContains: nil},
		{name: "assume_role_policy_json_string", fieldName: "assume_role_policy", value: `"blub"`, errContains: []string{
			`"assume_role_policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
		}},
		{name: "assume_role_policy_json_string_filename", fieldName: "assume_role_policy", value: `"../some-filename.json"`, errContains: []string{
			`"assume_role_policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
			"JSON-encoded filename",
		}},
		{name: "assume_role_policy_json_string_double_encoded", fieldName: "assume_role_policy", value: `"{\"Version\":\"...\"}"`, errContains: []string{
			`"assume_role_policy"`,
			"contains a JSON-encoded string, not a JSON-encoded object",
			"double-encoded",
		}},
		{name: "assume_role_policy_json_array", fieldName: "assume_role_policy", value: `[{"Statement":"value"}]`, errContains: []string{
			`"assume_role_policy"`,
			"contains a JSON array, not a JSON object",
		}},
		{name: "assume_role_policy_duplicate_keys", fieldName: "assume_role_policy", value: `{"a":"foo","a":"bar"}`, errContains: []string{
			`"assume_role_policy"`,
			"contains duplicate JSON keys",
			`duplicate key "a"`,
		}},
		// Starts with { but has invalid syntax -> NormalizeJsonString / json.SyntaxError.
		{name: "assume_role_policy_invalid_object_syntax", fieldName: "assume_role_policy", value: `{"def":}`, errContains: []string{
			`"assume_role_policy"`,
			"invalid JSON policy",
			"byte offset",
		}},

		// description: StringLenBetween(0, 1000)
		{name: "description_empty", fieldName: names.AttrDescription, value: "", errContains: nil},
		{name: "description_min", fieldName: names.AttrDescription, value: "a", errContains: nil},
		{name: "description_max", fieldName: names.AttrDescription, value: strings.Repeat("d", 1000), errContains: nil},
		{name: "description_unicode_ok", fieldName: names.AttrDescription, value: "角色描述-test_1.0 (ok)", errContains: nil},
		{name: "description_too_long", fieldName: names.AttrDescription, value: strings.Repeat("d", 1001), errContains: []string{
			"description",
			"expected length of description to be in the range (0 - 1000)",
		}},

		// name: validResourceName(64)
		{name: "name_valid_special_chars", fieldName: names.AttrName, value: "Role_-,.+@=123", errContains: nil},
		{name: "name_min", fieldName: names.AttrName, value: "a", errContains: nil},
		{name: "name_max", fieldName: names.AttrName, value: strings.Repeat("a", 64), errContains: nil},
		{name: "name_empty", fieldName: names.AttrName, value: "", errContains: []string{
			"expected length of name to be in the range (1 - 64)",
		}},
		{name: "name_too_long", fieldName: names.AttrName, value: strings.Repeat("a", 65), errContains: []string{
			"expected length of name to be in the range (1 - 64)",
		}},
		{name: "name_invalid_character", fieldName: names.AttrName, value: "invalid role!", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},

		// name_prefix: validResourceName(64 - UniqueIDSuffixLength)
		{name: "name_prefix_valid", fieldName: names.AttrNamePrefix, value: "RolePrefix_-,.+@=123", errContains: nil},
		{name: "name_prefix_too_long", fieldName: names.AttrNamePrefix, value: strings.Repeat("a", 64), errContains: []string{
			"expected length of name_prefix to be in the range",
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

// TestUnitIAMRole_maxSessionDurationSchema verifies that max_session_duration is Optional+Computed with no Terraform default,
// and validates the IntBetween(3600, 43200) boundaries (the service default is populated by Read rather than hard-coded in the schema).
func TestUnitIAMRole_maxSessionDurationSchema(t *testing.T) {
	t.Parallel()

	s := tfiam.ResourceRole().Schema["max_session_duration"]
	if s == nil {
		t.Fatal("expected max_session_duration schema")
	}
	if !s.Optional {
		t.Fatal("expected max_session_duration Optional")
	}
	if !s.Computed {
		t.Fatal("expected max_session_duration Computed")
	}
	if s.Default != nil {
		t.Fatalf("expected max_session_duration Default nil (server-side default), got %#v", s.Default)
	}
	if s.ValidateFunc == nil {
		t.Fatal("expected max_session_duration ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       int
		errContains []string
	}{
		{name: "min", value: 3600, errContains: nil},
		{name: "max", value: 43200, errContains: nil},
		{name: "below_min", value: 3599, errContains: []string{
			"expected max_session_duration to be in the range (3600 - 43200)",
		}},
		{name: "above_max", value: 43201, errContains: []string{
			"expected max_session_duration to be in the range (3600 - 43200)",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := s.ValidateFunc(tc.value, "max_session_duration")
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for max_session_duration=%d, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for max_session_duration=%d containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for max_session_duration missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TC-IAM-ROLE-01-Basic CRUD verifies role creation and import; when max_session_duration is omitted, Create does not send it,
// Read populates the service default (usually 3600), a repeated apply is Noop, and CheckDestroy covers destruction.
func TestAccIAMRole_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
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

// TC-IAM-ROLE-02-Description Update verifies description readback, replacement updates, and clearing to "", with remote and state expected to match.
func TestAccIAMRole_description(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_description(rName, "initial description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "initial description"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_description(rName, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "updated description"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
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
			// updated description -> "": clearing it should trigger an in-place Update (not ForceNew).
			{
				Config: testAccRoleConfig_description(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
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

// TC-IAM-ROLE-03-Session Duration: omit configuration to read the service default -> explicitly update to 7200;
// when the field is removed again, SDKv2 Optional+Computed retains the existing state/cloud value (does not revert to 3600), and plan is Noop.
func TestAccIAMRole_maxSessionDuration(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				// Omit max_session_duration: Create does not send it, and Read populates the service default.
				Config: testAccRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_maxSessionDuration(rName, 7200),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				// Remove the field from configuration: do not revert to the service default; retain 7200 without a persistent diff.
				Config: testAccRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
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

// TC-IAM-ROLE-04-AssumeRolePolicy Update verifies assume_role_policy updates through the UpdateAssumeRolePolicy path.
func TestAccIAMRole_assumeRolePolicy(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_assumeRolePolicy(rName, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_assumeRolePolicy(rName, "lambda.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("lambda.amazonaws.com")),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TC-IAM-ROLE-05-Update Path Split separately triggers UpdateRole (description+duration) and UpdateAssumeRolePolicy in one flow.
func TestAccIAMRole_updatePathSplit(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_attrs(rName, "initial", 3600, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "initial"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				// Change only description + max_session_duration -> UpdateRole.
				Config: testAccRoleConfig_attrs(rName, "updated-role", 7200, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "updated-role"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				// Change only assume_role_policy -> UpdateAssumeRolePolicy.
				Config: testAccRoleConfig_attrs(rName, "updated-role", 7200, "lambda.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "updated-role"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("lambda.amazonaws.com")),
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

// TC-IAM-ROLE-05-Update All Attributes updates description, max_session_duration, and assume_role_policy in one apply,
// expecting one Update to cover both UpdateRole and UpdateAssumeRolePolicy, with all three values matching on readback.
func TestAccIAMRole_updateAllAttributes(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_attrs(rName, "initial", 3600, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "initial"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_attrs(rName, "all-updated", 7200, "lambda.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "all-updated"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("lambda.amazonaws.com")),
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

// TC-IAM-ROLE-06-Name Generation verifies automatic generation of a unique name when name is unset.
func TestAccIAMRole_nameGenerated(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_nameGenerated(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleAttributesMatchCloud(resourceName, &role),
					acctest.CheckResourceAttrNameGenerated(resourceName, names.AttrName),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, "terraform-"),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
		},
	})
}

// TC-IAM-ROLE-06-Name Generation verifies that the generated name follows the prefix rule when name_prefix is used.
func TestAccIAMRole_namePrefix(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_namePrefix(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleAttributesMatchCloud(resourceName, &role),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrName, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
		},
	})
}

// TC-IAM-ROLE-06-Name Generation verifies that setting both name and name_prefix fails schema ConflictsWith validation.
func TestAccIAMRole_nameConflictsWithNamePrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccRoleConfig_nameAndNamePrefix(rName, acctest.ResourcePrefix),
				ExpectError: regexache.MustCompile(`(?s)("name": conflicts with name_prefix|"name_prefix": conflicts with name)`),
			},
		},
	})
}

// TC-IAM-ROLE-07-Disappears verifies that after remote role deletion and refresh, the resource is removed from state and a Create is planned.
func TestAccIAMRole_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceRole(), resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// TC-IAM-ROLE-08-Import verifies that an SDK-precreated role imported with terraform import has the same state as a normal creation.
// Follow ctyun_iam_role_policy_attachment importExisting: create the cloud object outside Terraform, then ImportStatePersist, and finish with an apply that has no persistent diff.
func TestAccIAMRole_importExisting(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"
	description := "sdk-created-role"
	assumePolicy := testAccRoleAssumePolicy("ec2.amazonaws.com")

	// SDK-precreated role: if import/apply fails before it enters Terraform state,
	// CheckDestroy (which scans only state) cannot remove it. Perform a fallback delete to avoid contaminating later tests.
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		conn := acctest.ProviderMeta(cleanupCtx, t).IAMClient(cleanupCtx)
		_, err := tfiam.FindRoleByName(cleanupCtx, conn, rName)
		if retry.NotFound(err) {
			return
		}
		if err != nil {
			t.Errorf("reading externally created IAM Role (%s) during cleanup: %s", rName, err)
			return
		}

		if _, err := conn.DeleteRole(cleanupCtx, &iam.DeleteRoleInput{
			RoleName: aws.String(rName),
		}); err != nil {
			t.Errorf("deleting externally created IAM Role (%s) during cleanup: %s", rName, err)
		}
	})

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				// Initialize the provider; the role is created only via SDK, bypassing Terraform Create.
				Config: `data "ctyun_iam_account_summary" "bootstrap" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExternalCreate(ctx, t, rName, description, assumePolicy),
				),
			},
			{
				Config:             testAccRoleConfig_description(rName, description),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateId:      rName,
				ImportStateCheck:   testAccRoleImportStateCheck(rName),
			},
			{
				Config: testAccRoleConfig_description(rName, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, description),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, assumePolicy),
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

// TC-IAM-ROLE-09-Description Then AssumeRolePolicy Split: update only description first (UpdateRole), then only assume_role_policy (UpdateAssumeRolePolicy).
func TestAccIAMRole_descriptionThenAssumeRolePolicySplit(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_descriptionAndPolicy(rName, "desc-v1", "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "desc-v1"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_descriptionAndPolicy(rName, "desc-v2", "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "desc-v2"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccRoleConfig_descriptionAndPolicy(rName, "desc-v2", "lambda.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, "desc-v2"),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("lambda.amazonaws.com")),
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

// TC-IAM-ROLE-10-SessionDuration Then Policy Split: update only max_session_duration first, then only assume_role_policy.
func TestAccIAMRole_sessionDurationThenPolicySplit(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_durationAndPolicy(rName, 3600, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
			{
				Config: testAccRoleConfig_durationAndPolicy(rName, 7200, "ec2.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccRoleConfig_durationAndPolicy(rName, 7200, "lambda.amazonaws.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 7200),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, testAccRoleAssumePolicy("lambda.amazonaws.com")),
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

// TC-IAM-ROLE-11-Equivalent Policy JSON: only reformat the JSON layout and field order of assume_role_policy; no perpetual diff is expected.
func TestAccIAMRole_equivalentAssumeRolePolicy(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_role.test"

	policy1 := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"sts:AssumeRole","Principal":{"Service":"ec2.amazonaws.com"}}]}`
	policy2 := `{
  "Statement": [
    {
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole",
      "Effect": "Allow"
    }
  ],
  "Version": "2012-10-17"
}`

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_assumePolicyRaw(rName, policy1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, policy1),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: testAccRoleConfig_assumePolicyRaw(rName, policy2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, rName),
					testAccCheckRoleDescription(resourceName, &role, ""),
					testAccCheckRoleMaxSessionDuration(resourceName, &role, 3600),
					testAccCheckRoleAssumeRolePolicy(resourceName, &role, policy1),
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

// TC-IAM-ROLE-12-Delete Detaches Policies verifies that when deleting a role with attached managed policies through
// Terraform, ResourceRole Delete first Lists/Detaches them and then calls DeleteRole; the policies remain managed by TF
// and their AttachmentCount drops to zero.
//
// Difference from TC-IAM-ROLE-07 Disappears:
//   - ROLE-07: simulates deletion outside Terraform and verifies that TF refresh detects the drift;
//   - ROLE-12: removes the role from the configuration and goes through normal apply/destroy, verifying the cleanup of
//     dependent attachments during deletion.
//
// The attachment is made via SDK Attach instead of being managed by ctyun_iam_role_policy_attachment, so that destroy does
// not delete the attachment first and bypass the automatic detach path inside Role Delete.
func TestAccIAMRole_deleteDetachesPolicies(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	var policy types.Policy
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	roleResourceName := "ctyun_iam_role.test"
	policyResourceName := "ctyun_iam_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckRoleDestroy(ctx, t),
			testAccCheckPolicyDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_roleAndManagedPolicy(rName, policyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, roleResourceName, &role),
					testAccCheckRoleStateMatchesCloud(roleResourceName, &role, rName),
					testAccCheckPolicyExists(ctx, t, policyResourceName, &policy),
					testAccCheckRolePolicyAttachmentExternalAttach(ctx, t, roleResourceName, policyResourceName),
					testAccCheckRolePolicyAttachmentList(ctx, t, rName, policyName),
					testAccCheckPolicyAttachmentCount(ctx, t, policyResourceName, 1),
				),
			},
			{
				// Remove the role from the configuration so that Terraform apply triggers ResourceRole Delete (the attachment still exists on the cloud at this point).
				Config: testAccRoleConfig_managedPolicyOnly(policyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleGone(ctx, t, rName),
					testAccCheckPolicyExists(ctx, t, policyResourceName, &policy),
					testAccCheckPolicyAttachmentCount(ctx, t, policyResourceName, 0),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(roleResourceName, plancheck.ResourceActionDestroy),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-ROLE-13-Name ForceNew verifies ForceNew replacement for name / name_prefix:
// 1) change name → Replace; 2) change name_prefix → Replace;
// 3) name → name_prefix → Replace; 4) name_prefix → name → Replace.
func TestAccIAMRole_nameForceNew(t *testing.T) {
	ctx := acctest.Context(t)
	var role types.Role
	var previousName string
	name1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name3 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	prefix1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix) + "-"
	prefix2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix) + "-"
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			// baseline: create with name
			{
				Config: testAccRoleConfig_basic(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, name1),
					testAccCheckRoleCaptureName(&role, &previousName),
				),
			},
			// 1) Change name → Destroy + Create
			{
				Config: testAccRoleConfig_basic(name2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, name2),
					testAccCheckRoleGoneByName(ctx, t, &previousName),
					testAccCheckRoleCaptureName(&role, &previousName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 3) name → name_prefix → Destroy + Create
			{
				Config: testAccRoleConfig_namePrefix(prefix1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleAttributesMatchCloud(resourceName, &role),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrName, prefix1),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, prefix1),
					testAccCheckRoleGoneByName(ctx, t, &previousName),
					testAccCheckRoleCaptureName(&role, &previousName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 2) Change name_prefix → Destroy + Create
			{
				Config: testAccRoleConfig_namePrefix(prefix2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleAttributesMatchCloud(resourceName, &role),
					acctest.CheckResourceAttrNameFromPrefix(resourceName, names.AttrName, prefix2),
					resource.TestCheckResourceAttr(resourceName, names.AttrNamePrefix, prefix2),
					testAccCheckRoleGoneByName(ctx, t, &previousName),
					testAccCheckRoleCaptureName(&role, &previousName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
			// 4) name_prefix → name → Destroy + Create
			{
				Config: testAccRoleConfig_basic(name3),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleExists(ctx, t, resourceName, &role),
					testAccCheckRoleStateMatchesCloud(resourceName, &role, name3),
					testAccCheckRoleGoneByName(ctx, t, &previousName),
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

func testAccCheckRoleDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_role" {
				continue
			}

			_, err := tfiam.FindRoleByName(ctx, conn, rs.Primary.ID)
			if retry.NotFound(err) {
				continue
			}
			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Role %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckRoleExists(ctx context.Context, t *testing.T, n string, v *types.Role) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no IAM Role ID is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := tfiam.FindRoleByName(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		*v = *output
		return nil
	}
}

func testAccCheckRoleGone(ctx context.Context, t *testing.T, roleName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := tfiam.FindRoleByName(ctx, conn, roleName)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("IAM Role %s still exists after delete", roleName)
	}
}

func testAccCheckRoleCaptureName(role *types.Role, name *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		*name = aws.ToString(role.RoleName)
		if *name == "" {
			return fmt.Errorf("expected role name to be set")
		}
		return nil
	}
}

func testAccCheckRoleGoneByName(ctx context.Context, t *testing.T, name *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if name == nil || *name == "" {
			return fmt.Errorf("previous role name is empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindRoleByName(ctx, conn, *name)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("IAM Role %s still exists after ForceNew replace", *name)
	}
}

// testAccCheckRoleExternalCreate pre-creates a cloud role via SDK CreateRole (bypassing Terraform Create).
func testAccCheckRoleExternalCreate(ctx context.Context, t *testing.T, name, description, assumeRolePolicy string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.CreateRole(ctx, &iam.CreateRoleInput{
			RoleName:                 aws.String(name),
			Description:              aws.String(description),
			AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
			MaxSessionDuration:       aws.Int32(3600),
		})
		if err != nil {
			return fmt.Errorf("externally creating IAM Role (%s): %w", name, err)
		}

		return nil
	}
}

func testAccRoleImportStateCheck(name string) resource.ImportStateCheckFunc {
	return func(is []*terraform.InstanceState) error {
		var rs *terraform.InstanceState
		for _, s := range is {
			if s.Ephemeral.Type == "ctyun_iam_role" {
				rs = s
				break
			}
		}
		if rs == nil {
			return fmt.Errorf("expected ctyun_iam_role in import state, got %d resources", len(is))
		}

		if rs.ID != name {
			return fmt.Errorf("bad import id: got %q, want %q", rs.ID, name)
		}
		if got := rs.Attributes[names.AttrName]; got != name {
			return fmt.Errorf("bad import name: got %q, want %q", got, name)
		}
		if rs.Attributes[names.AttrARN] == "" {
			return fmt.Errorf("expected arn to be set after import")
		}
		if rs.Attributes["unique_id"] == "" {
			return fmt.Errorf("expected unique_id to be set after import")
		}
		if rs.Attributes["create_date"] == "" {
			return fmt.Errorf("expected create_date to be set after import")
		}

		return nil
	}
}

// testAccCheckRoleStateMatchesCloud checks after Exists that the state of the given resourceName matches the cloud resource:
// id(=name), name, arn, unique_id, create_date.
func testAccCheckRoleStateMatchesCloud(resourceName string, role *types.Role, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.ToString(role.RoleName) != name {
			return fmt.Errorf("bad remote name: %s", aws.ToString(role.RoleName))
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}
		if rs.Type != "ctyun_iam_role" {
			return fmt.Errorf("%s: unexpected type %q, want ctyun_iam_role", resourceName, rs.Type)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}
		if rs.Primary.ID != name {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, name)
		}
		if got := rs.Primary.Attributes[names.AttrName]; got != name {
			return fmt.Errorf("bad state name: got %s, want %s", got, name)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], aws.ToString(role.Arn); got != want {
			return fmt.Errorf("bad arn: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["unique_id"], aws.ToString(role.RoleId); got != want {
			return fmt.Errorf("bad unique_id: got %s, want %s", got, want)
		}
		if role.CreateDate == nil {
			return fmt.Errorf("cloud create_date is nil")
		}
		if got, want := rs.Primary.Attributes["create_date"], role.CreateDate.Format(time.RFC3339); got != want {
			return fmt.Errorf("bad create_date: got %s, want %s", got, want)
		}

		return nil
	}
}

// testAccCheckRoleAttributesMatchCloud is used when name/name_prefix are auto-generated:
// it reads the cloud RoleName at Check execution time, avoiding early evaluation against an empty role at Compose time.
func testAccCheckRoleAttributesMatchCloud(resourceName string, role *types.Role) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return testAccCheckRoleStateMatchesCloud(resourceName, role, aws.ToString(role.RoleName))(s)
	}
}

func testAccCheckRoleDescription(resourceName string, role *types.Role, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got := aws.ToString(role.Description)
		if got != want {
			return fmt.Errorf("bad cloud description: got %q, want %q", got, want)
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		stateGot := rs.Primary.Attributes[names.AttrDescription]
		if stateGot != want {
			return fmt.Errorf("bad state description: got %q, want %q", stateGot, want)
		}
		return nil
	}
}

func testAccCheckRoleMaxSessionDuration(resourceName string, role *types.Role, want int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got := int(aws.ToInt32(role.MaxSessionDuration))
		if got != want {
			return fmt.Errorf("bad cloud max_session_duration: got %d, want %d", got, want)
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		wantStr := strconv.Itoa(want)
		stateGot := rs.Primary.Attributes["max_session_duration"]
		if stateGot != wantStr {
			return fmt.Errorf("bad state max_session_duration: got %s, want %s", stateGot, wantStr)
		}
		return nil
	}
}

func testAccCheckRoleAssumeRolePolicy(resourceName string, role *types.Role, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cloudPolicy, err := url.QueryUnescape(aws.ToString(role.AssumeRolePolicyDocument))
		if err != nil {
			return fmt.Errorf("unescaping cloud assume_role_policy: %w", err)
		}

		if !verify.PolicyStringsEquivalent(want, cloudPolicy) {
			return fmt.Errorf("bad cloud assume_role_policy: got %q, want equivalent to %q", cloudPolicy, want)
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		stateGot := rs.Primary.Attributes["assume_role_policy"]
		if !verify.PolicyStringsEquivalent(stateGot, cloudPolicy) {
			return fmt.Errorf("bad state assume_role_policy: got %q, want equivalent to cloud %q", stateGot, cloudPolicy)
		}
		return nil
	}
}

func testAccRoleConfig_basic(rName string) string {
	return testAccRoleConfig_assumeRolePolicy(rName, "ec2.amazonaws.com")
}

func testAccRoleConfig_description(rName, description string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  description        = %[2]q
  assume_role_policy = %[3]s
}
`, rName, description, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_maxSessionDuration(rName string, maxSessionDuration int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name                 = %[1]q
  max_session_duration = %[2]d
  assume_role_policy   = %[3]s
}
`, rName, maxSessionDuration, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_assumeRolePolicy(rName, service string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}
`, rName, testAccRoleAssumePolicyHCL(service))
}

func testAccRoleConfig_attrs(rName, description string, maxSessionDuration int, service string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name                 = %[1]q
  description          = %[2]q
  max_session_duration = %[3]d
  assume_role_policy   = %[4]s
}
`, rName, description, maxSessionDuration, testAccRoleAssumePolicyHCL(service))
}

func testAccRoleConfig_descriptionAndPolicy(rName, description, service string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  description        = %[2]q
  assume_role_policy = %[3]s
}
`, rName, description, testAccRoleAssumePolicyHCL(service))
}

func testAccRoleConfig_durationAndPolicy(rName string, maxSessionDuration int, service string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name                 = %[1]q
  max_session_duration = %[2]d
  assume_role_policy   = %[3]s
}
`, rName, maxSessionDuration, testAccRoleAssumePolicyHCL(service))
}

func testAccRoleConfig_assumePolicyRaw(rName, policyJSON string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]q
}
`, rName, policyJSON)
}

func testAccRoleConfig_nameGenerated() string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  assume_role_policy = %[1]s
}
`, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_namePrefix(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name_prefix        = %[1]q
  assume_role_policy = %[2]s
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_nameAndNamePrefix(name, namePrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  name_prefix        = %[2]q
  assume_role_policy = %[3]s
}
`, name, namePrefix, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_roleAndManagedPolicy(roleName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[3]s
}

resource "ctyun_iam_policy" "test" {
  name        = %[2]q
  description = "A test policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action   = ["iam:ChangePassword"]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}
`, roleName, policyName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleConfig_managedPolicyOnly(policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "A test policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action   = ["iam:ChangePassword"]
      Effect   = "Allow"
      Resource = "*"
    }]
  })
}
`, policyName)
}

func testAccRoleAssumePolicyHCL(service string) string {
	return fmt.Sprintf(`jsonencode(%s)`, testAccRoleAssumePolicy(service))
}

func testAccRoleAssumePolicy(service string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [{
    "Action": "sts:AssumeRole",
    "Principal": {
      "Service": %q
    },
    "Effect": "Allow"
  }]
}`, service)
}
