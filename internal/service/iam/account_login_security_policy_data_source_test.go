// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMAccountLoginSecurityPolicyDataSource_schema verifies the ctyun_iam_account_login_security_policy data source schema:
// it has no Required inputs; allow_single_users_simultaneous_login / lockout_duration /
// login_failed_times / period_with_login_failures / session_duration are all Computed.
func TestUnitIAMAccountLoginSecurityPolicyDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceAccountLoginSecurityPolicy()

	for _, name := range []string{
		"allow_single_users_simultaneous_login",
		"lockout_duration",
		"login_failed_times",
		"period_with_login_failures",
		"session_duration",
	} {
		attr, ok := dataSource.Schema[name]
		if !ok {
			t.Fatalf("expected %s schema", name)
		}
		if !attr.Computed {
			t.Fatalf("expected %s Computed", name)
		}
		if attr.Required {
			t.Fatalf("expected %s not Required", name)
		}
	}
}

// TestUnitIAMAccountLoginSecurityPolicyDataSource_flatten verifies flatten mapping for
// allow_single_users_simultaneous_login / lockout_duration / login_failed_times /
// period_with_login_failures / session_duration (including zero-valued fields).
func TestUnitIAMAccountLoginSecurityPolicyDataSource_flatten(t *testing.T) {
	t.Parallel()

	t.Run("non_zero", func(t *testing.T) {
		t.Parallel()

		policy := &tfiam.AccountLoginSecurityPolicyInput{
			AllowSingleUsersSimultaneousLogin: true,
			LockoutDuration:                   15,
			LoginFailedTimes:                  5,
			PeriodWithLoginFailures:           15,
			SessionDuration:                   30,
		}

		output := tfiam.FlattenAccountLoginSecurityPolicyForDataSource(policy)

		testCases := map[string]any{
			"allow_single_users_simultaneous_login": true,
			"lockout_duration":                      15,
			"login_failed_times":                    5,
			"period_with_login_failures":            15,
			"session_duration":                      30,
		}

		for k, want := range testCases {
			if got := output[k]; got != want {
				t.Fatalf("expected %s to be %v, got %v", k, want, got)
			}
		}
	})

	t.Run("zero", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FlattenAccountLoginSecurityPolicyForDataSource(&tfiam.AccountLoginSecurityPolicyInput{})

		testCases := map[string]any{
			"allow_single_users_simultaneous_login": false,
			"lockout_duration":                      0,
			"login_failed_times":                    0,
			"period_with_login_failures":            0,
			"session_duration":                      0,
		}

		for k, want := range testCases {
			if got := output[k]; got != want {
				t.Fatalf("expected %s to be %v, got %v", k, want, got)
			}
		}
	})
}

// TestUnitIAMAccountLoginSecurityPolicyDataSource_flatten_nil verifies that flatten returns an empty map for a nil policy.
func TestUnitIAMAccountLoginSecurityPolicyDataSource_flatten_nil(t *testing.T) {
	t.Parallel()

	output := tfiam.FlattenAccountLoginSecurityPolicyForDataSource(nil)

	if got, want := len(output), 0; got != want {
		t.Fatalf("expected %d attributes, got %d", want, got)
	}
}

// TestAccIAMAccountLoginSecurityPolicyDataSource_serial runs account-singleton DS ACC tests serially
// (the resource shares the same policy object, so parallel execution would overwrite state).
// It explicitly runs basic -> default -> consistency -> readStability: basic's CheckDestroy restores defaults first,
// then default runs (RunSerialTests1Level iterates maps in unspecified order and cannot guarantee this dependency).
func TestAccIAMAccountLoginSecurityPolicyDataSource_serial(t *testing.T) {
	t.Parallel()

	acctest.SkipIfNotRunningAcceptanceTests(t)

	testCases := []struct {
		Name string
		F    func(*testing.T)
	}{
		{"basic", testAccAccountLoginSecurityPolicyDataSource_basic},
		{"default", testAccAccountLoginSecurityPolicyDataSource_default},
		{"consistency", testAccAccountLoginSecurityPolicyDataSource_consistency},
		{"readStability", testAccAccountLoginSecurityPolicyDataSource_readStability},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, tc.F)
	}
}

// TC-IAM-DALSP-01-Basic Read verifies that after the resource sets the login security policy, the DS reads back the account singleton:
// it retains the resource AttrPair and explicitly asserts the basic configuration values (which equal defaults);
// DS state also matches cloud GetAccountLoginSecurityPolicy;
// CheckDestroy confirms that the cloud policy returns to defaults after destruction.
func testAccAccountLoginSecurityPolicyDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_login_security_policy.test"
	resourceName := "ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-login-security-policy"),
					resource.TestCheckResourceAttrPair(dataSourceName, "allow_single_users_simultaneous_login", resourceName, "allow_single_users_simultaneous_login"),
					resource.TestCheckResourceAttrPair(dataSourceName, "lockout_duration", resourceName, "lockout_duration"),
					resource.TestCheckResourceAttrPair(dataSourceName, "login_failed_times", resourceName, "login_failed_times"),
					resource.TestCheckResourceAttrPair(dataSourceName, "period_with_login_failures", resourceName, "period_with_login_failures"),
					resource.TestCheckResourceAttrPair(dataSourceName, "session_duration", resourceName, "session_duration"),
					resource.TestCheckResourceAttr(dataSourceName, "allow_single_users_simultaneous_login", testAccAccountLoginSecurityPolicyDefaultValues["allow_single_users_simultaneous_login"]),
					resource.TestCheckResourceAttr(dataSourceName, "lockout_duration", testAccAccountLoginSecurityPolicyDefaultValues["lockout_duration"]),
					resource.TestCheckResourceAttr(dataSourceName, "login_failed_times", testAccAccountLoginSecurityPolicyDefaultValues["login_failed_times"]),
					resource.TestCheckResourceAttr(dataSourceName, "period_with_login_failures", testAccAccountLoginSecurityPolicyDefaultValues["period_with_login_failures"]),
					resource.TestCheckResourceAttr(dataSourceName, "session_duration", testAccAccountLoginSecurityPolicyDefaultValues["session_duration"]),
				),
			},
		},
	})
}

// TC-IAM-DALSP-02-Default Readback verifies DS-only reading without a resource: the fixed id,
// all fields match cloud GetAccountLoginSecurityPolicy and equal service defaults
// (allow=true / lockout=15 / failed=5 / period=15 / session=30).
// In the serial suite, basic runs first and its CheckDestroy restores defaults; when run alone, the account must already use defaults.
func testAccAccountLoginSecurityPolicyDataSource_default(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_login_security_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_default,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-login-security-policy"),
					resource.TestCheckResourceAttr(dataSourceName, "allow_single_users_simultaneous_login", testAccAccountLoginSecurityPolicyDefaultValues["allow_single_users_simultaneous_login"]),
					resource.TestCheckResourceAttr(dataSourceName, "lockout_duration", testAccAccountLoginSecurityPolicyDefaultValues["lockout_duration"]),
					resource.TestCheckResourceAttr(dataSourceName, "login_failed_times", testAccAccountLoginSecurityPolicyDefaultValues["login_failed_times"]),
					resource.TestCheckResourceAttr(dataSourceName, "period_with_login_failures", testAccAccountLoginSecurityPolicyDefaultValues["period_with_login_failures"]),
					resource.TestCheckResourceAttr(dataSourceName, "session_duration", testAccAccountLoginSecurityPolicyDefaultValues["session_duration"]),
				),
			},
		},
	})
}

// TC-IAM-DALSP-03-Resource To DataSource Consistency verifies that after the resource modifies the policy,
// the DS immediately reads it back, retains the resource AttrPair, and matches the cloud; the second step explicitly asserts updated expected values;
// CheckDestroy confirms that the cloud policy returns to defaults after destruction.
func testAccAccountLoginSecurityPolicyDataSource_consistency(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_login_security_policy.test"
	resourceName := "ctyun_iam_account_login_security_policy.test"

	pairChecks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-login-security-policy"),
		resource.TestCheckResourceAttrPair(dataSourceName, "allow_single_users_simultaneous_login", resourceName, "allow_single_users_simultaneous_login"),
		resource.TestCheckResourceAttrPair(dataSourceName, "lockout_duration", resourceName, "lockout_duration"),
		resource.TestCheckResourceAttrPair(dataSourceName, "login_failed_times", resourceName, "login_failed_times"),
		resource.TestCheckResourceAttrPair(dataSourceName, "period_with_login_failures", resourceName, "period_with_login_failures"),
		resource.TestCheckResourceAttrPair(dataSourceName, "session_duration", resourceName, "session_duration"),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_basic,
				Check:  pairChecks,
			},
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_modified,
				Check: resource.ComposeAggregateTestCheckFunc(
					pairChecks,
					resource.TestCheckResourceAttr(dataSourceName, "allow_single_users_simultaneous_login", acctest.CtFalse),
					resource.TestCheckResourceAttr(dataSourceName, "lockout_duration", "20"),
					resource.TestCheckResourceAttr(dataSourceName, "login_failed_times", "7"),
					resource.TestCheckResourceAttr(dataSourceName, "period_with_login_failures", "25"),
					resource.TestCheckResourceAttr(dataSourceName, "session_duration", "25"),
				),
			},
		},
	})
}

// TC-IAM-DALSP-04-Read Stability verifies two consecutive applies with identical configuration: each DS state
// matches cloud GetAccountLoginSecurityPolicy; the second apply has no diff (PreApply / PostApplyPostRefresh);
// CheckDestroy confirms that the cloud policy returns to defaults after destruction.
func testAccAccountLoginSecurityPolicyDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_login_security_policy.test"

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-login-security-policy"),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountLoginSecurityPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_basic,
				Check:  checks,
			},
			{
				Config: testAccAccountLoginSecurityPolicyDataSourceConfig_basic,
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

// testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud verifies DS state
// against cloud GetAccountLoginSecurityPolicy without relying on managed resource state:
// it compares the fixed id and five policy fields (bool/int values are compared as state strings).
func testAccCheckAccountLoginSecurityPolicyDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}
		if rs.Primary.ID != "iam-account-login-security-policy" {
			return fmt.Errorf("bad state id: got %s, want iam-account-login-security-policy", rs.Primary.ID)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountLoginSecurityPolicy(ctx, conn)
		if err != nil {
			return fmt.Errorf("reading IAM Account Login Security Policy: %w", err)
		}
		if policy == nil {
			return fmt.Errorf("IAM Account Login Security Policy not found in cloud")
		}

		cloudValues := testAccAccountLoginSecurityPolicyValues(
			policy.AllowSingleUsersSimultaneousLogin,
			policy.LockoutDuration,
			policy.LoginFailedTimes,
			policy.PeriodWithLoginFailures,
			policy.SessionDuration,
		)
		for attribute, want := range cloudValues {
			if got := rs.Primary.Attributes[attribute]; got != want {
				return fmt.Errorf("bad %s: state=%q GetAccountLoginSecurityPolicy=%q", attribute, got, want)
			}
		}

		return nil
	}
}

var testAccAccountLoginSecurityPolicyDataSourceConfig_basic = acctest.ConfigCompose(
	testAccAccountLoginSecurityPolicyConfig_basic,
	`
data "ctyun_iam_account_login_security_policy" "test" {
  depends_on = [ctyun_iam_account_login_security_policy.test]
}
`,
)

var testAccAccountLoginSecurityPolicyDataSourceConfig_modified = acctest.ConfigCompose(
	testAccAccountLoginSecurityPolicyConfig_modified,
	`
data "ctyun_iam_account_login_security_policy" "test" {
  depends_on = [ctyun_iam_account_login_security_policy.test]
}
`,
)

const testAccAccountLoginSecurityPolicyDataSourceConfig_default = `
data "ctyun_iam_account_login_security_policy" "test" {}
`
