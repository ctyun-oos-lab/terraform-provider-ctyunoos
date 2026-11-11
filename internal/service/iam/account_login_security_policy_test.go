// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-IAM-ALSP-01-Basic verifies basic CRUD for the account login security policy: creation, remote/state reading, import, in-place update, and restoration of the service default policy after destruction.
func TestAccIAMAccountLoginSecurityPolicy_Basic(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, names.AttrID, "iam-account-login-security-policy"),
					testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyDefaultValues),
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
				Config: testAccAccountLoginSecurityPolicyConfig_modified,
				Check:  testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyModifiedValues),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-ALSP-02-BoundaryValidation verifies service-side success boundaries for four numeric fields: the lower-bound combination and each field's upper bound can be applied and read back consistently.
func TestAccIAMAccountLoginSecurityPolicy_BoundaryValidation(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 15, 5, 15, 10),
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 60, 5, 15, 10),
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 15, 10, 15, 10),
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 15, 5, 60, 10),
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 15, 5, 15, 30),
		},
	})
}

// TC-IAM-ALSP-03-RemoteDeleteRecovery verifies that after the policy is deleted remotely and reverts to defaults, Terraform restores declared values with Update.
func TestAccIAMAccountLoginSecurityPolicy_RemoteDeleteRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyModifiedValues),
					testAccCheckAccountLoginSecurityPolicyRemoteDelete(ctx, t),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccAccountLoginSecurityPolicyConfig_modified,
				Check:  testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyModifiedValues),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-ALSP-04-DefaultReadback verifies that after remote deletion reverts the policy to defaults, the complete default configuration is read back consistently without a persistent diff.
func TestAccIAMAccountLoginSecurityPolicy_DefaultReadback(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyConfig_modified,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyModifiedValues),
					testAccCheckAccountLoginSecurityPolicyRemoteDelete(ctx, t),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccAccountLoginSecurityPolicyConfig_basic,
				Check:  testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyDefaultValues),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-ALSP-06-RemoteDeleteToDefault verifies that after external deletion, RefreshState replaces the former non-default state with service defaults.
func TestAccIAMAccountLoginSecurityPolicy_RemoteDeleteToDefault(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyConfig_modified,
				Check:  testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyModifiedValues),
			},
			{
				PreConfig: func() {
					if err := testAccDeleteAccountLoginSecurityPolicyAndVerifyDefaults(ctx, t); err != nil {
						t.Fatal(err)
					}
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, testAccAccountLoginSecurityPolicyDefaultValues),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TC-IAM-ALSP-08-BoundaryMatrix verifies that when all four numeric fields are simultaneously at their lower or upper bounds, both the service and state read back correctly.
func TestAccIAMAccountLoginSecurityPolicy_BoundaryMatrix(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 15, 5, 15, 10),
			testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx, t, resourceName, 60, 10, 60, 30),
		},
	})
}

// TC-IAM-ALSP-02/08 verifies local validation for the four numeric fields and rejects out-of-range values.
func TestUnitIAMAccountLoginSecurityPolicy_Validation(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceAccountLoginSecurityPolicy()
	for _, attribute := range []string{
		"allow_single_users_simultaneous_login",
		"lockout_duration",
		"login_failed_times",
		"period_with_login_failures",
		"session_duration",
	} {
		schema, ok := resource.Schema[attribute]
		if !ok || schema == nil {
			t.Errorf("expected %s schema", attribute)
			continue
		}

		if !schema.Optional || !schema.Computed || schema.Default != nil {
			t.Errorf("expected %s to be Optional+Computed without a Default", attribute)
		}
	}

	testCases := []struct {
		name              string
		lockoutDuration   int
		loginFailedTimes  int
		failurePeriod     int
		sessionDuration   int
		wantInvalidFields []string
	}{
		{name: "lockout_duration_too_low", lockoutDuration: 14, loginFailedTimes: 5, failurePeriod: 15, sessionDuration: 10, wantInvalidFields: []string{"lockout_duration"}},
		{name: "lockout_duration_too_high", lockoutDuration: 61, loginFailedTimes: 5, failurePeriod: 15, sessionDuration: 10, wantInvalidFields: []string{"lockout_duration"}},
		{name: "login_failed_times_too_low", lockoutDuration: 15, loginFailedTimes: 4, failurePeriod: 15, sessionDuration: 10, wantInvalidFields: []string{"login_failed_times"}},
		{name: "login_failed_times_too_high", lockoutDuration: 15, loginFailedTimes: 11, failurePeriod: 15, sessionDuration: 10, wantInvalidFields: []string{"login_failed_times"}},
		{name: "period_with_login_failures_too_low", lockoutDuration: 15, loginFailedTimes: 5, failurePeriod: 14, sessionDuration: 10, wantInvalidFields: []string{"period_with_login_failures"}},
		{name: "period_with_login_failures_too_high", lockoutDuration: 15, loginFailedTimes: 5, failurePeriod: 61, sessionDuration: 10, wantInvalidFields: []string{"period_with_login_failures"}},
		{name: "session_duration_too_low", lockoutDuration: 15, loginFailedTimes: 5, failurePeriod: 15, sessionDuration: 9, wantInvalidFields: []string{"session_duration"}},
		{name: "session_duration_too_high", lockoutDuration: 15, loginFailedTimes: 5, failurePeriod: 15, sessionDuration: 31, wantInvalidFields: []string{"session_duration"}},
		{name: "all_fields_below_bounds", lockoutDuration: 14, loginFailedTimes: 4, failurePeriod: 14, sessionDuration: 9, wantInvalidFields: []string{"lockout_duration", "login_failed_times", "period_with_login_failures", "session_duration"}},
		{name: "all_fields_above_bounds", lockoutDuration: 61, loginFailedTimes: 11, failurePeriod: 61, sessionDuration: 31, wantInvalidFields: []string{"lockout_duration", "login_failed_times", "period_with_login_failures", "session_duration"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]int{
				"lockout_duration":           tc.lockoutDuration,
				"login_failed_times":         tc.loginFailedTimes,
				"period_with_login_failures": tc.failurePeriod,
				"session_duration":           tc.sessionDuration,
			}

			invalidFields := map[string]bool{}
			for fieldName, value := range values {
				validate := resource.Schema[fieldName].ValidateFunc
				if validate == nil {
					t.Fatalf("expected %s validation", fieldName)
				}

				if _, errs := validate(value, fieldName); len(errs) > 0 {
					invalidFields[fieldName] = true
				}
			}

			if got, want := len(invalidFields), len(tc.wantInvalidFields); got != want {
				t.Fatalf("invalid field count = %d, want %d: %v", got, want, invalidFields)
			}
			for _, fieldName := range tc.wantInvalidFields {
				if !invalidFields[fieldName] {
					t.Fatalf("expected %s to fail validation; invalid fields: %v", fieldName, invalidFields)
				}
			}
		})
	}
}

// Verifies that AccountLoginSecurityPolicyUpdateValues sends only explicitly configured fields and preserves an explicit false.
func TestUnitIAMAccountLoginSecurityPolicy_UpdateValues(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceAccountLoginSecurityPolicy()
	testCases := map[string]struct {
		config map[string]any
		want   map[string]string
	}{
		"configured_true": {
			config: map[string]any{
				"allow_single_users_simultaneous_login": true,
			},
			want: map[string]string{
				"AllowSingleUsersSimultaneousLogin": "true",
			},
		},
		"configured_false_and_numeric_values": {
			config: map[string]any{
				"allow_single_users_simultaneous_login": false,
				"lockout_duration":                      15,
				"login_failed_times":                    5,
				"period_with_login_failures":            15,
				"session_duration":                      10,
			},
			want: map[string]string{
				"AllowSingleUsersSimultaneousLogin": "false",
				"LockoutDuration":                   "15",
				"LoginFailedTimes":                  "5",
				"PeriodWithLoginFailures":           "15",
				"SessionDuration":                   "10",
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
			values := tfiam.AccountLoginSecurityPolicyUpdateValues(d)

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

func testAccCheckAccountLoginSecurityPolicyState(ctx context.Context, t *testing.T, resourceName string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No IAM Account Login Security Policy ID is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountLoginSecurityPolicy(ctx, conn)
		if err != nil {
			return err
		}

		remoteValues := testAccAccountLoginSecurityPolicyValues(
			policy.AllowSingleUsersSimultaneousLogin,
			policy.LockoutDuration,
			policy.LoginFailedTimes,
			policy.PeriodWithLoginFailures,
			policy.SessionDuration,
		)
		for attribute, wantValue := range want {
			if got := remoteValues[attribute]; got != wantValue {
				return fmt.Errorf("remote %s = %q, want %q", attribute, got, wantValue)
			}
			if got := rs.Primary.Attributes[attribute]; got != wantValue {
				return fmt.Errorf("state %s = %q, want %q", attribute, got, wantValue)
			}
		}

		return nil
	}
}

func testAccCheckAccountLoginSecurityPolicyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountLoginSecurityPolicy(ctx, conn)
		if err != nil {
			return fmt.Errorf("reading IAM Account Login Security Policy after destroy: %w", err)
		}

		remoteValues := testAccAccountLoginSecurityPolicyValues(
			policy.AllowSingleUsersSimultaneousLogin,
			policy.LockoutDuration,
			policy.LoginFailedTimes,
			policy.PeriodWithLoginFailures,
			policy.SessionDuration,
		)
		for attribute, want := range testAccAccountLoginSecurityPolicyDefaultValues {
			if got := remoteValues[attribute]; got != want {
				return fmt.Errorf("remote %s = %q after destroy, want default %q", attribute, got, want)
			}
		}

		return nil
	}
}

// testAccCheckAccountLoginSecurityPolicyRemoteDelete directly invokes the OOS IAM Delete action to simulate out-of-band deletion.
func testAccCheckAccountLoginSecurityPolicyRemoteDelete(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return testAccDeleteAccountLoginSecurityPolicyAndVerifyDefaults(ctx, t)
	}
}

func testAccDeleteAccountLoginSecurityPolicyAndVerifyDefaults(ctx context.Context, t *testing.T) error {
	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	if _, err := tfiam.OOSIAMQuery(ctx, conn, "DeleteAccountLoginSecurityPolicy", nil); err != nil {
		return fmt.Errorf("externally deleting IAM Account Login Security Policy: %w", err)
	}

	policy, err := tfiam.FindAccountLoginSecurityPolicy(ctx, conn)
	if err != nil {
		return fmt.Errorf("reading IAM Account Login Security Policy after external delete: %w", err)
	}

	remoteValues := testAccAccountLoginSecurityPolicyValues(
		policy.AllowSingleUsersSimultaneousLogin,
		policy.LockoutDuration,
		policy.LoginFailedTimes,
		policy.PeriodWithLoginFailures,
		policy.SessionDuration,
	)
	for attribute, want := range testAccAccountLoginSecurityPolicyDefaultValues {
		if got := remoteValues[attribute]; got != want {
			return fmt.Errorf("remote %s = %q after external delete, want default %q", attribute, got, want)
		}
	}

	return nil
}

func testAccAccountLoginSecurityPolicyValues(allowSimultaneousLogin bool, lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration int) map[string]string {
	return map[string]string{
		"allow_single_users_simultaneous_login": strconv.FormatBool(allowSimultaneousLogin),
		"lockout_duration":                      strconv.Itoa(lockoutDuration),
		"login_failed_times":                    strconv.Itoa(loginFailedTimes),
		"period_with_login_failures":            strconv.Itoa(periodWithLoginFailures),
		"session_duration":                      strconv.Itoa(sessionDuration),
	}
}

func testAccAccountLoginSecurityPolicyNumericBoundsStep(ctx context.Context, t *testing.T, resourceName string, lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration int) resource.TestStep {
	want := testAccAccountLoginSecurityPolicyValues(true, lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration)

	return resource.TestStep{
		Config: testAccAccountLoginSecurityPolicyConfig_numericBounds(lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration),
		Check:  testAccCheckAccountLoginSecurityPolicyState(ctx, t, resourceName, want),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PostApplyPostRefresh: []plancheck.PlanCheck{
				plancheck.ExpectEmptyPlan(),
			},
		},
	}
}

var testAccAccountLoginSecurityPolicyDefaultValues = map[string]string{
	"allow_single_users_simultaneous_login": "true",
	"lockout_duration":                      "15",
	"login_failed_times":                    "5",
	"period_with_login_failures":            "15",
	"session_duration":                      "30",
}

var testAccAccountLoginSecurityPolicyModifiedValues = map[string]string{
	"allow_single_users_simultaneous_login": "false",
	"lockout_duration":                      "20",
	"login_failed_times":                    "7",
	"period_with_login_failures":            "25",
	"session_duration":                      "25",
}

const testAccAccountLoginSecurityPolicyConfig_basic = `
resource "ctyun_iam_account_login_security_policy" "test" {
  allow_single_users_simultaneous_login = true
  lockout_duration                      = 15
  login_failed_times                    = 5
  period_with_login_failures            = 15
  session_duration                      = 30
}
`

const testAccAccountLoginSecurityPolicyConfig_modified = `
resource "ctyun_iam_account_login_security_policy" "test" {
  allow_single_users_simultaneous_login = false
  lockout_duration                      = 20
  login_failed_times                    = 7
  period_with_login_failures            = 25
  session_duration                      = 25
}
`

func testAccAccountLoginSecurityPolicyConfig_numericBounds(lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_account_login_security_policy" "test" {
  allow_single_users_simultaneous_login = true
  lockout_duration                      = %[1]d
  login_failed_times                    = %[2]d
  period_with_login_failures            = %[3]d
  session_duration                      = %[4]d
}
`, lockoutDuration, loginFailedTimes, periodWithLoginFailures, sessionDuration)
}
