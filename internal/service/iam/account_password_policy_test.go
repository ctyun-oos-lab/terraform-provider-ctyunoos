// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-IAM-APP-01-Basic verifies basic CRUD for the account password policy: set the policy, read each field, import state, update in place, and confirm service defaults after destruction.
func TestAccIAMAccountPasswordPolicy_Basic(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountPasswordPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyDefaultValues),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAccountPasswordPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyModifiedValues),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-APP-02-MinLengthBounds verifies service-side success boundaries for minimum_password_length: 8 and 128 apply successfully and read back consistently.
func TestAccIAMAccountPasswordPolicy_MinimumPasswordLengthBounds(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_minimumPasswordLength(8),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"minimum_password_length": "8",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_minimumPasswordLength(128),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"minimum_password_length": "128",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TC-IAM-APP-03-MaxAgeBounds verifies service-side success boundaries for max_password_age: 0 (never expires), 1, and 1095 apply successfully and read back consistently.
func TestAccIAMAccountPasswordPolicy_MaxPasswordAgeBounds(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_maxPasswordAge(0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"max_password_age": "0",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_maxPasswordAge(1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"max_password_age": "1",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_maxPasswordAge(1095),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"max_password_age": "1095",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TC-IAM-APP-04-ReusePreventionBounds verifies service-side success boundaries for password_reuse_prevention: 0 (reuse allowed) and 24 apply successfully and read back consistently.
func TestAccIAMAccountPasswordPolicy_PasswordReusePreventionBounds(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_passwordReusePrevention(0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"password_reuse_prevention": "0",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_passwordReusePrevention(24),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, map[string]string{
						"password_reuse_prevention": "24",
					}),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TC-IAM-APP-05-ComplexityFlags verifies independent updates and readback of six boolean fields: each step enables only one field, then the next or final step switches it back to false.
func TestAccIAMAccountPasswordPolicy_ComplexityFlags(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"
	disabled := accountPasswordPolicyFlagValues{}

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, disabled),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{requireLowercaseCharacters: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{requireUppercaseCharacters: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{requireNumbers: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{requireSymbols: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{allowUsersToChangePassword: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, accountPasswordPolicyFlagValues{hardExpiry: true}),
			testAccAccountPasswordPolicyFlagStep(ctx, t, resourceName, &policy, disabled),
		},
	})
}

// TC-IAM-APP-06-RemoteDeleteRecovery verifies that a subsequent apply restores declared state after remote policy deletion: step one deletes the policy through the SDK in Check and expects a non-empty plan; step two converges to the original configuration and reads back consistently.
func TestAccIAMAccountPasswordPolicy_RemoteDeleteRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyModifiedValues),
					testAccCheckAccountPasswordPolicyRemoteDelete(ctx, t),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyModifiedValues),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-APP-07/09-DefaultRestoreIdempotence verifies the flow "remote delete -> read defaults -> apply default configuration -> plan again with no diff".
func TestAccIAMAccountPasswordPolicy_DefaultRestoreIdempotence(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyRemoteDelete(ctx, t),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_defaults,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyDefaultValues),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-APP-10-BoundaryMatrix verifies boundary combinations for three numeric fields: lower- and upper-bound combinations apply successfully and read back consistently remotely and in state. Out-of-range combinations are covered by unit tests.
func TestAccIAMAccountPasswordPolicy_BoundaryMatrix(t *testing.T) {
	ctx := acctest.Context(t)
	var policy awstypes.PasswordPolicy
	resourceName := "ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyConfig_numericBounds(8, 0, 0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyNumericValues(8, 0, 0)),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccAccountPasswordPolicyConfig_numericBounds(128, 1095, 24),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, &policy),
					testAccCheckAccountPasswordPolicyState(resourceName, &policy, testAccAccountPasswordPolicyNumericValues(128, 1095, 24)),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TC-IAM-APP-02/03/04/10 verifies local validation for the three numeric fields and rejects out-of-range values.
func TestUnitIAMAccountPasswordPolicy_Validation(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceAccountPasswordPolicy()
	for _, attribute := range []string{
		"allow_users_to_change_password",
		"hard_expiry",
		"max_password_age",
		"minimum_password_length",
		"password_reuse_prevention",
		"require_lowercase_characters",
		"require_numbers",
		"require_symbols",
		"require_uppercase_characters",
	} {
		schema := resource.Schema[attribute]
		if !schema.Optional || !schema.Computed || schema.Default != nil {
			t.Errorf("expected %s to be Optional+Computed without a Default", attribute)
		}
	}

	testCases := []struct {
		name              string
		minimumLength     int
		maxAge            int
		reusePrevention   int
		wantInvalidFields []string
	}{
		{name: "lower_bound_combination", minimumLength: 8, maxAge: 0, reusePrevention: 0},
		{name: "upper_bound_combination", minimumLength: 128, maxAge: 1095, reusePrevention: 24},
		{name: "minimum_password_length_too_low", minimumLength: 7, maxAge: 0, reusePrevention: 0, wantInvalidFields: []string{"minimum_password_length"}},
		{name: "minimum_password_length_too_high", minimumLength: 129, maxAge: 0, reusePrevention: 0, wantInvalidFields: []string{"minimum_password_length"}},
		{name: "max_password_age_too_low", minimumLength: 8, maxAge: -1, reusePrevention: 0, wantInvalidFields: []string{"max_password_age"}},
		{name: "max_password_age_too_high", minimumLength: 8, maxAge: 1096, reusePrevention: 0, wantInvalidFields: []string{"max_password_age"}},
		{name: "password_reuse_prevention_too_low", minimumLength: 8, maxAge: 0, reusePrevention: -1, wantInvalidFields: []string{"password_reuse_prevention"}},
		{name: "password_reuse_prevention_too_high", minimumLength: 8, maxAge: 0, reusePrevention: 25, wantInvalidFields: []string{"password_reuse_prevention"}},
		{name: "all_fields_below_bounds", minimumLength: 7, maxAge: -1, reusePrevention: -1, wantInvalidFields: []string{"minimum_password_length", "max_password_age", "password_reuse_prevention"}},
		{name: "all_fields_above_bounds", minimumLength: 129, maxAge: 1096, reusePrevention: 25, wantInvalidFields: []string{"minimum_password_length", "max_password_age", "password_reuse_prevention"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]int{
				"minimum_password_length":   tc.minimumLength,
				"max_password_age":          tc.maxAge,
				"password_reuse_prevention": tc.reusePrevention,
			}

			invalidFields := make([]string, 0, len(values))
			for fieldName, value := range values {
				validate := resource.Schema[fieldName].ValidateFunc
				if validate == nil {
					t.Fatalf("expected %s validation", fieldName)
				}

				if _, errs := validate(value, fieldName); len(errs) > 0 {
					invalidFields = append(invalidFields, fieldName)
				}
			}

			slices.Sort(invalidFields)
			wantInvalidFields := append([]string{}, tc.wantInvalidFields...)
			slices.Sort(wantInvalidFields)
			if diff := cmp.Diff(wantInvalidFields, invalidFields); diff != "" {
				t.Fatalf("unexpected invalid fields (-want +got):\n%s", diff)
			}
		})
	}
}

// Verifies that AccountPasswordPolicyUpdateValues maps only explicitly configured fields to UpdateAccountPasswordPolicy request parameters and preserves explicit false/0 values.
func TestUnitIAMAccountPasswordPolicy_UpdateValues(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceAccountPasswordPolicy()
	testCases := map[string]struct {
		config map[string]any
		want   map[string]string
	}{
		"configured_true": {
			config: map[string]any{
				"allow_users_to_change_password": true,
				"hard_expiry":                    true,
				"require_lowercase_characters":   true,
				"require_numbers":                true,
				"require_symbols":                true,
				"require_uppercase_characters":   true,
			},
			want: map[string]string{
				"AllowUsersToChangePassword": "true",
				"HardExpiry":                 "true",
				"RequireLowercaseCharacters": "true",
				"RequireNumbers":             "true",
				"RequireSymbols":             "true",
				"RequireUppercaseCharacters": "true",
			},
		},
		"configured_false_and_zero_values": {
			config: map[string]any{
				"allow_users_to_change_password": false,
				"hard_expiry":                    false,
				"max_password_age":               0,
				"minimum_password_length":        9,
				"password_reuse_prevention":      0,
				"require_lowercase_characters":   false,
				"require_numbers":                false,
				"require_symbols":                false,
				"require_uppercase_characters":   false,
			},
			want: map[string]string{
				"AllowUsersToChangePassword": "false",
				"HardExpiry":                 "false",
				"MaxPasswordAge":             "0",
				"MinimumPasswordLength":      "9",
				"PasswordReusePrevention":    "0",
				"RequireLowercaseCharacters": "false",
				"RequireNumbers":             "false",
				"RequireSymbols":             "false",
				"RequireUppercaseCharacters": "false",
			},
		},
		"omitted_fields": {
			config: map[string]any{},
			want:   map[string]string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resource.Schema, tc.config)
			values := tfiam.AccountPasswordPolicyUpdateValues(d)

			if got, want := len(values), len(tc.want); got != want {
				t.Fatalf("request values has %d fields, want %d: %v", got, want, values)
			}
			for key, want := range tc.want {
				if got := values.Get(key); got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func testAccCheckAccountPasswordPolicyExists(ctx context.Context, t *testing.T, n string, v *awstypes.PasswordPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No IAM Account Password Policy ID is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindAccountPasswordPolicy(ctx, conn)

		if err != nil {
			return err
		}

		*v = *output

		return nil
	}
}

func testAccCheckAccountPasswordPolicyState(resourceName string, policy *awstypes.PasswordPolicy, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if err := testAccCheckAccountPasswordPolicyRemoteValues(policy, want); err != nil {
			return err
		}

		for attribute, wantValue := range want {
			if got := rs.Primary.Attributes[attribute]; got != wantValue {
				return fmt.Errorf("state %s = %q, want %q", attribute, got, wantValue)
			}
		}

		return nil
	}
}

func testAccCheckAccountPasswordPolicyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountPasswordPolicy(ctx, conn)
		if err != nil {
			return fmt.Errorf("reading IAM Account Password Policy after destroy: %w", err)
		}

		if err := testAccCheckAccountPasswordPolicyRemoteValues(policy, testAccAccountPasswordPolicyDefaultValues); err != nil {
			return fmt.Errorf("IAM Account Password Policy was not restored to defaults after destroy: %w", err)
		}

		return nil
	}
}

func testAccCheckAccountPasswordPolicyRemoteValues(policy *awstypes.PasswordPolicy, want map[string]string) error {
	if policy == nil {
		return fmt.Errorf("remote password policy is nil")
	}

	got := map[string]string{
		"allow_users_to_change_password": strconv.FormatBool(policy.AllowUsersToChangePassword),
		"expire_passwords":               strconv.FormatBool(policy.ExpirePasswords),
		"hard_expiry":                    strconv.FormatBool(aws.ToBool(policy.HardExpiry)),
		"max_password_age":               strconv.Itoa(int(aws.ToInt32(policy.MaxPasswordAge))),
		"minimum_password_length":        strconv.Itoa(int(aws.ToInt32(policy.MinimumPasswordLength))),
		"password_reuse_prevention":      strconv.Itoa(int(aws.ToInt32(policy.PasswordReusePrevention))),
		"require_lowercase_characters":   strconv.FormatBool(policy.RequireLowercaseCharacters),
		"require_numbers":                strconv.FormatBool(policy.RequireNumbers),
		"require_symbols":                strconv.FormatBool(policy.RequireSymbols),
		"require_uppercase_characters":   strconv.FormatBool(policy.RequireUppercaseCharacters),
	}

	for attribute, wantValue := range want {
		if gotValue := got[attribute]; gotValue != wantValue {
			return fmt.Errorf("remote %s = %q, want %q", attribute, gotValue, wantValue)
		}
	}

	return nil
}

// testAccCheckAccountPasswordPolicyRemoteDelete deletes the account password policy remotely through the SDK to simulate deletion outside Terraform.
func testAccCheckAccountPasswordPolicyRemoteDelete(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.DeleteAccountPasswordPolicy(ctx, &iam.DeleteAccountPasswordPolicyInput{})
		if err != nil {
			return fmt.Errorf("externally deleting IAM Account Password Policy: %w", err)
		}

		policy, err := tfiam.FindAccountPasswordPolicy(ctx, conn)
		if err != nil {
			return fmt.Errorf("reading IAM Account Password Policy after external delete: %w", err)
		}

		if err := testAccCheckAccountPasswordPolicyRemoteValues(policy, testAccAccountPasswordPolicyDefaultValues); err != nil {
			return fmt.Errorf("IAM Account Password Policy was not restored to defaults after external delete: %w", err)
		}

		return nil
	}
}

type accountPasswordPolicyFlagValues struct {
	allowUsersToChangePassword bool
	hardExpiry                 bool
	requireLowercaseCharacters bool
	requireNumbers             bool
	requireSymbols             bool
	requireUppercaseCharacters bool
}

func (v accountPasswordPolicyFlagValues) stateValues() map[string]string {
	return map[string]string{
		"allow_users_to_change_password": strconv.FormatBool(v.allowUsersToChangePassword),
		"hard_expiry":                    strconv.FormatBool(v.hardExpiry),
		"require_lowercase_characters":   strconv.FormatBool(v.requireLowercaseCharacters),
		"require_numbers":                strconv.FormatBool(v.requireNumbers),
		"require_symbols":                strconv.FormatBool(v.requireSymbols),
		"require_uppercase_characters":   strconv.FormatBool(v.requireUppercaseCharacters),
	}
}

func testAccAccountPasswordPolicyFlagStep(ctx context.Context, t *testing.T, resourceName string, policy *awstypes.PasswordPolicy, flags accountPasswordPolicyFlagValues) resource.TestStep {
	return resource.TestStep{
		Config: testAccAccountPasswordPolicyConfig_complexityFlags(flags),
		Check: resource.ComposeTestCheckFunc(
			testAccCheckAccountPasswordPolicyExists(ctx, t, resourceName, policy),
			testAccCheckAccountPasswordPolicyState(resourceName, policy, flags.stateValues()),
		),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		},
	}
}

var testAccAccountPasswordPolicyDefaultValues = map[string]string{
	"allow_users_to_change_password": "true",
	"expire_passwords":               "false",
	"hard_expiry":                    "false",
	"max_password_age":               "0",
	"minimum_password_length":        "8",
	"password_reuse_prevention":      "0",
	"require_lowercase_characters":   "true",
	"require_numbers":                "true",
	"require_symbols":                "false",
	"require_uppercase_characters":   "false",
}

var testAccAccountPasswordPolicyModifiedValues = map[string]string{
	"allow_users_to_change_password": "true",
	"expire_passwords":               "false",
	"hard_expiry":                    "false",
	"max_password_age":               "0",
	"minimum_password_length":        "9",
	"password_reuse_prevention":      "0",
	"require_lowercase_characters":   "true",
	"require_numbers":                "true",
	"require_symbols":                "true",
	"require_uppercase_characters":   "true",
}

const testAccAccountPasswordPolicyConfig_basic = `
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = true
  minimum_password_length        = 8
  require_numbers                = true
}
`

const testAccAccountPasswordPolicyConfig_modified = `
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = true
  minimum_password_length        = 9
  require_numbers                = true
  require_symbols                = true
  require_uppercase_characters   = true
}
`

const testAccAccountPasswordPolicyConfig_defaults = `
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = true
  hard_expiry                    = false
  max_password_age               = 0
  minimum_password_length        = 8
  password_reuse_prevention      = 0
  require_lowercase_characters   = true
  require_numbers                = true
  require_symbols                = false
  require_uppercase_characters   = false
}
`

func testAccAccountPasswordPolicyConfig_minimumPasswordLength(length int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  minimum_password_length = %[1]d
}
`, length)
}

func testAccAccountPasswordPolicyConfig_maxPasswordAge(age int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  max_password_age = %[1]d
}
`, age)
}

func testAccAccountPasswordPolicyConfig_passwordReusePrevention(count int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  password_reuse_prevention = %[1]d
}
`, count)
}

func testAccAccountPasswordPolicyConfig_numericBounds(minLength, maxAge, reusePrevention int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  max_password_age          = %[1]d
  minimum_password_length   = %[2]d
  password_reuse_prevention = %[3]d
}
`, maxAge, minLength, reusePrevention)
}

func testAccAccountPasswordPolicyNumericValues(minLength, maxAge, reusePrevention int) map[string]string {
	return map[string]string{
		"minimum_password_length":   strconv.Itoa(minLength),
		"max_password_age":          strconv.Itoa(maxAge),
		"password_reuse_prevention": strconv.Itoa(reusePrevention),
	}
}

func testAccAccountPasswordPolicyConfig_complexityFlags(flags accountPasswordPolicyFlagValues) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = %[1]t
  hard_expiry                    = %[2]t
  require_lowercase_characters   = %[3]t
  require_numbers                = %[4]t
  require_symbols                = %[5]t
  require_uppercase_characters   = %[6]t
}
`, flags.allowUsersToChangePassword, flags.hardExpiry, flags.requireLowercaseCharacters, flags.requireNumbers, flags.requireSymbols, flags.requireUppercaseCharacters)
}
