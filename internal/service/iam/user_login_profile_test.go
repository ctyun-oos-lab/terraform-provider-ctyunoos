// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Verifies that the default password generation logic produces passwords compliant with the default policy at both the minimum and maximum length boundaries.
func TestUnitIAMUserLoginProfile_generatePassword(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		length int
	}{
		"default_min": {length: 8},
		"default_max": {length: 128},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p, err := tfiam.GeneratePassword(tc.length)
			if err != nil {
				t.Fatal(err)
			}
			if len(p) != tc.length {
				t.Fatalf("expected a %d character password, got %d: %q", tc.length, len(p), p)
			}
			if strings.Trim(p, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
				t.Fatalf("expected generated password to contain only lowercase letters and numbers, got: %q", p)
			}
			if !tfiam.CheckPwdPolicy([]byte(p)) {
				t.Fatalf("expected generated password to match OOS default policy, got: %q", p)
			}
		})
	}
}

// Verifies that when the account password policy explicitly configures a minimum length plus uppercase/lowercase, digit, and symbol requirements, the generated password satisfies the policy and contains every required character class.
func TestUnitIAMUserLoginProfile_generatePasswordWithPolicy(t *testing.T) {
	t.Parallel()

	policy := &types.PasswordPolicy{
		MinimumPasswordLength:      aws.Int32(12),
		RequireLowercaseCharacters: true,
		RequireUppercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             true,
	}

	p, err := tfiam.GeneratePasswordWithPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 12 {
		t.Fatalf("expected password length from account policy, got %d: %q", len(p), p)
	}
	if !tfiam.CheckPwdPolicyWithPolicy([]byte(p), policy) {
		t.Fatalf("expected generated password to match account policy, got: %q", p)
	}
	for name, charset := range map[string]string{
		"lowercase": "abcdefghijklmnopqrstuvwxyz",
		"uppercase": "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"number":    "0123456789",
		"symbol":    "!@#$%^&*()_+-=[]{}|'",
	} {
		if !strings.ContainsAny(p, charset) {
			t.Fatalf("expected generated password to contain %s character, got: %q", name, p)
		}
	}
}

// Verifies that a non-empty partial account password policy is passed through as-is, and that character classes not required by it do not fall back to the default complexity.
func TestUnitIAMUserLoginProfile_generatePasswordWithPolicy_partialPolicy(t *testing.T) {
	t.Parallel()

	policy := &types.PasswordPolicy{
		MinimumPasswordLength: aws.Int32(10),
		RequireSymbols:        true,
	}

	p, err := tfiam.GeneratePasswordWithPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 10 {
		t.Fatalf("expected password length from partial policy, got %d: %q", len(p), p)
	}
	if !tfiam.CheckPwdPolicyWithPolicy([]byte(p), policy) {
		t.Fatalf("expected generated password to match partial policy, got: %q", p)
	}
	if strings.Trim(p, "!@#$%^&*()_+-=[]{}|'") != "" {
		t.Fatalf("expected partial-policy password to contain symbols only, got: %q", p)
	}
}

// Verifies that an empty or nil policy falls back to the default password length and default complexity requirements.
func TestUnitIAMUserLoginProfile_generatePasswordWithPolicy_default(t *testing.T) {
	t.Parallel()

	for name, policy := range map[string]*types.PasswordPolicy{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p, err := tfiam.GeneratePasswordWithPolicy(policy)
			if err != nil {
				t.Fatal(err)
			}
			if len(p) != 8 {
				t.Fatalf("expected default password length 8, got %d: %q", len(p), p)
			}
			if strings.Trim(p, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
				t.Fatalf("expected default generated password to contain only lowercase letters and numbers, got: %q", p)
			}
			if !tfiam.CheckPwdPolicy([]byte(p)) {
				t.Fatalf("expected generated password to match default policy, got: %q", p)
			}
		})
	}
}

// Verifies that a missing, zero, or negative policy minimum length falls back to the default length of 8.
func TestUnitIAMUserLoginProfile_generatePasswordWithPolicy_minimumLengthFallback(t *testing.T) {
	t.Parallel()

	for name, policy := range map[string]*types.PasswordPolicy{
		"zero":     {MinimumPasswordLength: aws.Int32(0)},
		"negative": {MinimumPasswordLength: aws.Int32(-1)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p, err := tfiam.GeneratePasswordWithPolicy(policy)
			if err != nil {
				t.Fatal(err)
			}
			if len(p) != 8 {
				t.Fatalf("expected fallback password length 8, got %d: %q", len(p), p)
			}
			if !tfiam.CheckPwdPolicyWithPolicy([]byte(p), policy) {
				t.Fatalf("expected generated password to match policy fallback behavior, got: %q", p)
			}
		})
	}
}

// Verifies that when the number of character classes required by the policy exceeds the minimum length, the generation logic automatically raises the password length to accommodate all required characters.
func TestUnitIAMUserLoginProfile_generatePasswordWithPolicy_adjustsLengthForRequiredCharsets(t *testing.T) {
	t.Parallel()

	policy := &types.PasswordPolicy{
		MinimumPasswordLength:      aws.Int32(1),
		RequireLowercaseCharacters: true,
		RequireUppercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             true,
	}

	p, err := tfiam.GeneratePasswordWithPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) < 4 {
		t.Fatalf("expected generated password length to be adjusted to at least 4, got %d: %q", len(p), p)
	}
	if !tfiam.CheckPwdPolicyWithPolicy([]byte(p), policy) {
		t.Fatalf("expected generated password to match policy, got: %q", p)
	}
}

// Verifies that the CreateLoginProfile input correctly encodes both the default and explicit password_reset_required values.
func TestUnitIAMUserLoginProfile_userLoginProfileCreateValues(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceUserLoginProfile()
	for _, tc := range []struct {
		name  string
		input map[string]any
		want  string
	}{
		{
			name: "default_false",
			input: map[string]any{
				"user": "test-user",
			},
			want: "false",
		},
		{
			name: "explicit_true",
			input: map[string]any{
				"user":                    "test-user",
				"password_reset_required": true,
			},
			want: "true",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resource.Schema, tc.input)
			values := tfiam.UserLoginProfileCreateValues(d, "Abcd1234")

			for key, want := range map[string]string{
				"UserName":              "test-user",
				"Password":              "Abcd1234",
				"PasswordResetRequired": tc.want,
			} {
				if got := values.Get(key); got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

// Verifies the schema definition of the user parameter and its length and character-set validation rules.
func TestUnitIAMUserLoginProfile_UserValidation(t *testing.T) {
	t.Parallel()

	userSchema := tfiam.ResourceUserLoginProfile().Schema["user"]
	if !userSchema.Required {
		t.Fatal("expected user to be required")
	}
	if userSchema.Optional {
		t.Fatal("expected user not to be optional")
	}
	if userSchema.Computed {
		t.Fatal("expected user not to be computed")
	}
	if !userSchema.ForceNew {
		t.Fatal("expected user changes to force a new login profile")
	}

	validate := userSchema.ValidateFunc
	if validate == nil {
		t.Fatal("expected user validation")
	}

	for _, tc := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "minimum_length", value: "a", wantErr: false},
		{name: "allowed_characters", value: "User_-,.+@=123", wantErr: false},
		{name: "maximum_length", value: strings.Repeat("a", 64), wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 65), wantErr: true},
		{name: "space", value: "invalid user", wantErr: true},
		{name: "slash", value: "invalid/user", wantErr: true},
		{name: "exclamation_mark", value: "invalid!user", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, validationErrors := validate(tc.value, "user")
			if gotErr := len(validationErrors) > 0; gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; errors: %v", gotErr, tc.wantErr, validationErrors)
			}
		})
	}
}

// Verifies that under the default password policy, password validation accepts valid samples containing only lowercase letters and digits, and rejects invalid samples missing any default element or containing extra characters.
func TestUnitIAMUserLoginProfile_passwordPolicyCheck(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		pass  string
		valid bool
	}{
		{pass: "abcd1234", valid: true},
		{pass: "abcdefgh", valid: false},
		{pass: "12345678", valid: false},
		{pass: "abcd123A", valid: false},
		{pass: "abcd123#", valid: false},
	} {
		t.Run(tc.pass, func(t *testing.T) {
			t.Parallel()

			valid := tfiam.CheckPwdPolicy([]byte(tc.pass))
			if valid != tc.valid {
				t.Fatalf("expected %q to be valid==%t, got %t", tc.pass, tc.valid, valid)
			}
			if valid && strings.Trim(tc.pass, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
				t.Fatalf("expected %q to contain only lowercase letters and numbers", tc.pass)
			}
		})
	}
}

// TC-IAM-LP-01 Basic Create verifies that after creating a login profile, the remote GetLoginProfile is readable, user and password are correctly back-filled in state, and import works.
func TestAccIAMUserLoginProfile_basicCreate(t *testing.T) {
	ctx := acctest.Context(t)
	var conf iam.GetLoginProfileOutput

	resourceName := "ctyun_iam_user_login_profile.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserLoginProfileDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserLoginProfileConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserLoginProfileExists(ctx, t, resourceName, &conf),
					testAccCheckUserLoginProfileID(&conf, rName),
					testAccCheckUserLoginProfileUser(&conf, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, "user", rName),
					testAccCheckUserLoginProfilePasswordMatchesDefaultPolicy(ctx, t, resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					if len(s) != 1 {
						return fmt.Errorf("expected 1 state: %#v", s)
					}

					attrs := s[0].Attributes
					if got := attrs[names.AttrPassword]; got != "" {
						return fmt.Errorf("password = %q, want empty", got)
					}
					if got := attrs["password_reset_required"]; got != acctest.CtFalse {
						return fmt.Errorf("password_reset_required = %q, want %q", got, acctest.CtFalse)
					}

					return nil
				},
				ImportStateVerifyIgnore: []string{
					names.AttrPassword,
					"password_reset_required",
				},
			},
		},
	})
}

// TC-IAM-LP-01 Basic Create additionally verifies that when an account-level password policy is configured, the password generated during login profile creation complies with that policy.
func TestAccIAMUserLoginProfile_customPasswordPolicy(t *testing.T) {
	ctx := acctest.Context(t)
	var loginProfile iam.GetLoginProfileOutput
	var passwordPolicy types.PasswordPolicy

	const (
		loginProfileResourceName    = "ctyun_iam_user_login_profile.test"
		passwordPolicyResourceName  = "ctyun_iam_account_password_policy.test"
		customPasswordMinimumLength = 12
	)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	customPolicy := &types.PasswordPolicy{
		MinimumPasswordLength:      aws.Int32(customPasswordMinimumLength),
		RequireLowercaseCharacters: true,
		RequireUppercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             true,
	}
	customPolicyStateValues := map[string]string{
		"minimum_password_length":      "12",
		"require_lowercase_characters": acctest.CtTrue,
		"require_uppercase_characters": acctest.CtTrue,
		"require_numbers":              acctest.CtTrue,
		"require_symbols":              acctest.CtTrue,
	}

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckUserLoginProfileDestroy(ctx, t),
			testAccCheckAccountPasswordPolicyDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccUserLoginProfileConfig_customPasswordPolicy(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountPasswordPolicyExists(ctx, t, passwordPolicyResourceName, &passwordPolicy),
					testAccCheckAccountPasswordPolicyState(passwordPolicyResourceName, &passwordPolicy, customPolicyStateValues),
					testAccCheckUserLoginProfileExists(ctx, t, loginProfileResourceName, &loginProfile),
					testAccCheckUserLoginProfileID(&loginProfile, rName),
					testAccCheckUserLoginProfileUser(&loginProfile, rName),
					resource.TestCheckResourceAttr(loginProfileResourceName, "user", rName),
					testAccCheckUserLoginProfilePasswordMatchesPolicy(loginProfileResourceName, customPolicy),
				),
			},
		},
	})
}

// Verifies that creating a login profile for a non-existent IAM User returns a clear resource-not-found error.
func TestAccIAMUserLoginProfile_missingUser(t *testing.T) {
	ctx := acctest.Context(t)
	missingUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-login-profile-missing")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserLoginProfileConfig_missingUser(missingUserName),
				ExpectError: regexache.MustCompile(`(?i)(couldn't find\s+resource|NoSuchEntity|StatusCode:\s*404)`),
			},
		},
	})
}

// TC-IAM-LP-03 Password Complexity Coupling verifies that when the account password policy enables digit, uppercase, lowercase, and symbol requirements with a minimum length of 8, password validation judges success samples and various failure samples as expected.
func TestUnitIAMUserLoginProfile_passwordPolicyComplexityCoupling(t *testing.T) {
	t.Parallel()

	policy := &types.PasswordPolicy{
		MinimumPasswordLength:      aws.Int32(8),
		RequireLowercaseCharacters: true,
		RequireUppercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             true,
	}

	for _, tc := range []struct {
		name  string
		pass  string
		valid bool
	}{
		{name: "success", pass: "Abcd1234#", valid: true},
		{name: "missing_number", pass: "abcdefgh#", valid: false},
		{name: "missing_uppercase", pass: "abcd1234#", valid: false},
		{name: "missing_lowercase", pass: "ABCD1234#", valid: false},
		{name: "missing_symbol", pass: "Abcd1234", valid: false},
		{name: "too_short", pass: "Ab1#xyz", valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			valid := tfiam.CheckPwdPolicyWithPolicy([]byte(tc.pass), policy)
			if valid != tc.valid {
				t.Fatalf("expected %q to be valid==%t, got %t", tc.pass, tc.valid, valid)
			}
		})
	}
}

// TC-IAM-LP-04 Reset Required verifies that password_reset_required reads back false from the server when not explicitly configured, and that false -> true -> false transitions all trigger ForceNew replacement.
func TestAccIAMUserLoginProfile_passwordResetRequired(t *testing.T) {
	ctx := acctest.Context(t)
	var conf iam.GetLoginProfileOutput

	resourceName := "ctyun_iam_user_login_profile.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserLoginProfileDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserLoginProfileConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserLoginProfileExists(ctx, t, resourceName, &conf),
					testAccCheckUserLoginProfileID(&conf, rName),
					testAccCheckUserLoginProfileUser(&conf, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, "password_reset_required", acctest.CtFalse),
					testAccCheckUserLoginProfilePasswordMatchesDefaultPolicy(ctx, t, resourceName),
				),
			},
			{
				Config: testAccUserLoginProfileConfig_passwordResetRequired(rName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserLoginProfileExists(ctx, t, resourceName, &conf),
					testAccCheckUserLoginProfileID(&conf, rName),
					testAccCheckUserLoginProfileUser(&conf, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, "password_reset_required", acctest.CtTrue),
					testAccCheckUserLoginProfilePasswordMatchesDefaultPolicy(ctx, t, resourceName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccUserLoginProfileConfig_passwordResetRequired(rName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserLoginProfileExists(ctx, t, resourceName, &conf),
					testAccCheckUserLoginProfileID(&conf, rName),
					testAccCheckUserLoginProfileUser(&conf, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, "password_reset_required", acctest.CtFalse),
					testAccCheckUserLoginProfilePasswordMatchesDefaultPolicy(ctx, t, resourceName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestUnitIAMUserLoginProfile_passwordPolicyCheck_partialRequirements verifies that when the policy enables only some character-type requirements, password validation enforces only the enabled ones.
func TestUnitIAMUserLoginProfile_passwordPolicyCheck_partialRequirements(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		policy *types.PasswordPolicy
		pass   string
		valid  bool
	}{
		{
			name: "lower_and_number_success",
			policy: &types.PasswordPolicy{
				MinimumPasswordLength:      aws.Int32(8),
				RequireLowercaseCharacters: true,
				RequireNumbers:             true,
			},
			pass:  "abcd1234",
			valid: true,
		},
		{
			name: "lower_and_number_missing_number",
			policy: &types.PasswordPolicy{
				MinimumPasswordLength:      aws.Int32(8),
				RequireLowercaseCharacters: true,
				RequireNumbers:             true,
			},
			pass:  "abcdefgh",
			valid: false,
		},
		{
			name: "symbol_only_success",
			policy: &types.PasswordPolicy{
				MinimumPasswordLength: aws.Int32(5),
				RequireSymbols:        true,
			},
			pass:  "abcd#",
			valid: true,
		},
		{
			name: "symbol_only_missing_symbol",
			policy: &types.PasswordPolicy{
				MinimumPasswordLength: aws.Int32(5),
				RequireSymbols:        true,
			},
			pass:  "abcd1",
			valid: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			valid := tfiam.CheckPwdPolicyWithPolicy([]byte(tc.pass), tc.policy)
			if valid != tc.valid {
				t.Fatalf("expected %q to be valid==%t, got %t", tc.pass, tc.valid, valid)
			}
		})
	}
}

// TestUnitIAMUserLoginProfile_passwordPolicyCheck_lengthOnly verifies that when the policy only restricts the minimum length without requiring specific character types, password validation checks only the length.
func TestUnitIAMUserLoginProfile_passwordPolicyCheck_lengthOnly(t *testing.T) {
	t.Parallel()

	policy := &types.PasswordPolicy{
		MinimumPasswordLength: aws.Int32(10),
	}

	for _, tc := range []struct {
		name  string
		pass  string
		valid bool
	}{
		{name: "too_short", pass: "abc123", valid: false},
		{name: "letters_only", pass: "abcdefghij", valid: true},
		{name: "mixed_charset", pass: "ABCD1234#x", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			valid := tfiam.CheckPwdPolicyWithPolicy([]byte(tc.pass), policy)
			if valid != tc.valid {
				t.Fatalf("expected %q to be valid==%t, got %t", tc.pass, tc.valid, valid)
			}
		})
	}
}

// TC-IAM-LP-06 Disappears deletes the login profile outside Terraform via the IAM SDK, then verifies that refresh removes it from state and produces a Create plan.
func TestAccIAMUserLoginProfile_disappearsAfterExternalDelete(t *testing.T) {
	ctx := acctest.Context(t)
	var conf iam.GetLoginProfileOutput

	resourceName := "ctyun_iam_user_login_profile.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserLoginProfileDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserLoginProfileConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserLoginProfileExists(ctx, t, resourceName, &conf),
					testAccCheckUserLoginProfileDisappearsExternally(ctx, t, resourceName),
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
					testAccCheckUserLoginProfileRemovedFromState(resourceName),
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

func testAccCheckUserLoginProfileDisappearsExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		userName := rs.Primary.ID
		if userName == "" {
			return fmt.Errorf("IAM User Login Profile (%s) has no user name", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.DeleteLoginProfile(ctx, &iam.DeleteLoginProfileInput{UserName: aws.String(userName)})
		if err != nil {
			return fmt.Errorf("externally deleting IAM User Login Profile for User (%s): %w", userName, err)
		}

		_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := conn.GetLoginProfile(ctx, &iam.GetLoginProfileInput{UserName: aws.String(userName)})

			if errs.IsA[*types.NoSuchEntityException](err) ||
				tfawserr.ErrCodeEquals(err, (&types.NoSuchEntityException{}).ErrorCode()) ||
				tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			return fmt.Errorf("waiting for IAM User Login Profile for User (%s) to disappear: %w", userName, err)
		}

		return nil
	}
}

func testAccCheckUserLoginProfileRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM User Login Profile resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

func testAccCheckUserLoginProfileDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user_login_profile" {
				continue
			}

			_, err := tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
				_, err := conn.GetLoginProfile(ctx, &iam.GetLoginProfileInput{
					UserName: aws.String(rs.Primary.ID),
				})

				if errs.IsA[*types.NoSuchEntityException](err) ||
					tfawserr.ErrCodeEquals(err, (&types.NoSuchEntityException{}).ErrorCode()) ||
					tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
					return nil, &retry.NotFoundError{LastError: err}
				}

				return nil, err
			})

			if err != nil {
				return fmt.Errorf("IAM User Login Profile (%s) still exists", rs.Primary.ID)
			}
		}

		return nil
	}
}

func testAccCheckUserLoginProfileExists(ctx context.Context, t *testing.T, n string, res *iam.GetLoginProfileOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return errors.New("no UserName is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		resp, err := conn.GetLoginProfile(ctx, &iam.GetLoginProfileInput{
			UserName: aws.String(rs.Primary.ID),
		})
		if err != nil {
			if errs.IsA[*types.NoSuchEntityException](err) ||
				tfawserr.ErrCodeEquals(err, (&types.NoSuchEntityException{}).ErrorCode()) ||
				tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
				return fmt.Errorf("IAM User Login Profile for User (%s) was not found: %w", rs.Primary.ID, err)
			}

			return fmt.Errorf("reading IAM User Login Profile for User (%s): %w", rs.Primary.ID, err)
		}
		if resp == nil || resp.LoginProfile == nil {
			return fmt.Errorf("reading IAM User Login Profile for User (%s): empty response", rs.Primary.ID)
		}

		*res = *resp
		return nil
	}
}

func testAccCheckUserLoginProfileID(conf *iam.GetLoginProfileOutput, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if conf == nil || conf.LoginProfile == nil {
			return errors.New("login profile response is empty")
		}

		got := aws.ToString(conf.LoginProfile.UserName)
		if got != want {
			return fmt.Errorf("bad remote id: got %s, want %s", got, want)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user_login_profile" {
				continue
			}

			stateID := rs.Primary.ID
			if stateID != want {
				return fmt.Errorf("bad state id: got %s, want %s", stateID, want)
			}
			return nil
		}

		return errors.New("ctyun_iam_user_login_profile not found in state")
	}
}

func testAccCheckUserLoginProfileUser(conf *iam.GetLoginProfileOutput, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if conf == nil || conf.LoginProfile == nil {
			return errors.New("login profile response is empty")
		}

		got := aws.ToString(conf.LoginProfile.UserName)
		if got != want {
			return fmt.Errorf("bad user: got %s, want %s", got, want)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user_login_profile" {
				continue
			}

			stateUser := rs.Primary.Attributes["user"]
			if stateUser != want {
				return fmt.Errorf("bad state user: got %s, want %s", stateUser, want)
			}
			return nil
		}

		return errors.New("ctyun_iam_user_login_profile not found in state")
	}
}

func testAccCheckUserLoginProfilePasswordMatchesDefaultPolicy(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		password := rs.Primary.Attributes[names.AttrPassword]
		if password == "" {
			return errors.New("password is empty")
		}

		policy, err := tfiam.FindAccountPasswordPolicy(ctx, acctest.ProviderMeta(ctx, t).IAMClient(ctx))
		if retry.NotFound(err) {
			if !tfiam.CheckPwdPolicy([]byte(password)) {
				return fmt.Errorf("password %q does not match default policy", password)
			}

			return nil
		}
		if err != nil {
			return fmt.Errorf("reading account password policy: %w", err)
		}

		if !tfiam.CheckPwdPolicyWithPolicy([]byte(password), policy) {
			return fmt.Errorf("password %q does not match account password policy", password)
		}

		return nil
	}
}

func testAccCheckUserLoginProfilePasswordMatchesPolicy(resourceName string, policy *types.PasswordPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		password := rs.Primary.Attributes[names.AttrPassword]
		if password == "" {
			return errors.New("password is empty")
		}

		if !tfiam.CheckPwdPolicyWithPolicy([]byte(password), policy) {
			return fmt.Errorf("password %q does not match account password policy", password)
		}

		return nil
	}
}

func testAccUserLoginProfileConfig_base(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}
`, rName)
}

func testAccUserLoginProfileConfig_basic(rName string) string {
	return testAccUserLoginProfileConfig_base(rName) + `
resource "ctyun_iam_user_login_profile" "test" {
  user = ctyun_iam_user.test.name
}
`
}

func testAccUserLoginProfileConfig_passwordResetRequired(rName string, passwordResetRequired bool) string {
	return testAccUserLoginProfileConfig_base(rName) + fmt.Sprintf(`
resource "ctyun_iam_user_login_profile" "test" {
  user                    = ctyun_iam_user.test.name
  password_reset_required = %[1]t
}
`, passwordResetRequired)
}

func testAccUserLoginProfileConfig_customPasswordPolicy(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_password_policy" "test" {
  minimum_password_length      = 12
  require_lowercase_characters = true
  require_uppercase_characters = true
  require_numbers              = true
  require_symbols              = true
}

resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_user_login_profile" "test" {
  user = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_account_password_policy.test]
}
`, rName)
}

func testAccUserLoginProfileConfig_missingUser(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user_login_profile" "test" {
  user = %[1]q
}
`, userName)
}
