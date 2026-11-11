// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMAccountPasswordPolicyDataSource_schema verifies the ctyun_iam_account_password_policy data source schema:
// it has no Required inputs; allow_users_to_change_password / expire_passwords / hard_expiry /
// max_password_age / minimum_password_length / password_reuse_prevention /
// require_* and related fields are all Computed.
func TestUnitIAMAccountPasswordPolicyDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceAccountPasswordPolicy()

	for _, name := range []string{
		"allow_users_to_change_password",
		"expire_passwords",
		"hard_expiry",
		"max_password_age",
		"minimum_password_length",
		"password_reuse_prevention",
		"require_lowercase_characters",
		"require_numbers",
		"require_symbols",
		"require_uppercase_characters",
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

// TestUnitIAMAccountPasswordPolicyDataSource_flatten verifies flatten mapping for all password-policy fields.
func TestUnitIAMAccountPasswordPolicyDataSource_flatten(t *testing.T) {
	t.Parallel()

	policy := &awstypes.PasswordPolicy{
		AllowUsersToChangePassword: true,
		ExpirePasswords:            false,
		HardExpiry:                 aws.Bool(false),
		MaxPasswordAge:             aws.Int32(0),
		MinimumPasswordLength:      aws.Int32(8),
		PasswordReusePrevention:    aws.Int32(0),
		RequireLowercaseCharacters: true,
		RequireNumbers:             true,
		RequireSymbols:             true,
		RequireUppercaseCharacters: true,
	}

	output := tfiam.FlattenAccountPasswordPolicyForDataSource(policy)

	testCases := map[string]any{
		"allow_users_to_change_password": true,
		"expire_passwords":               false,
		"hard_expiry":                    false,
		"max_password_age":               0,
		"minimum_password_length":        8,
		"password_reuse_prevention":      0,
		"require_lowercase_characters":   true,
		"require_numbers":                true,
		"require_symbols":                true,
		"require_uppercase_characters":   true,
	}

	for k, want := range testCases {
		if got := output[k]; got != want {
			t.Fatalf("expected %s to be %v, got %v", k, want, got)
		}
	}
}

// TestUnitIAMAccountPasswordPolicyDataSource_flatten_nil verifies that flatten returns an empty map for a nil policy.
func TestUnitIAMAccountPasswordPolicyDataSource_flatten_nil(t *testing.T) {
	t.Parallel()

	output := tfiam.FlattenAccountPasswordPolicyForDataSource(nil)

	if got, want := len(output), 0; got != want {
		t.Fatalf("expected %d attributes, got %d", want, got)
	}
}

// TestAccIAMAccountPasswordPolicyDataSource_serial runs account-singleton DS ACC tests serially
// (the resource shares the same policy object, so parallel execution would overwrite state).
// It explicitly runs basic -> default -> consistency -> readStability: basic's CheckDestroy restores defaults first,
// then default runs (RunSerialTests1Level iterates maps in unspecified order and cannot guarantee this dependency).
func TestAccIAMAccountPasswordPolicyDataSource_serial(t *testing.T) {
	t.Parallel()

	acctest.SkipIfNotRunningAcceptanceTests(t)

	testCases := []struct {
		Name string
		F    func(*testing.T)
	}{
		{"basic", testAccAccountPasswordPolicyDataSource_basic},
		{"default", testAccAccountPasswordPolicyDataSource_default},
		{"consistency", testAccAccountPasswordPolicyDataSource_consistency},
		{"readStability", testAccAccountPasswordPolicyDataSource_readStability},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, tc.F)
	}
}

// TC-IAM-DAPP-01-Basic Read verifies that after the resource sets every configurable field to a non-default value, the DS reads back the account singleton:
// it matches cloud GetAccountPasswordPolicy and explicitly asserts the non-default expected values;
// CheckDestroy confirms that the cloud policy returns to defaults after destruction.
// expire_passwords is Computed (derived as true when max_password_age > 0) and cannot be configured directly.
func testAccAccountPasswordPolicyDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountPasswordPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-password-policy"),
					resource.TestCheckResourceAttr(dataSourceName, "allow_users_to_change_password", testAccAccountPasswordPolicyDataSourceNonDefaultValues["allow_users_to_change_password"]),
					resource.TestCheckResourceAttr(dataSourceName, "expire_passwords", testAccAccountPasswordPolicyDataSourceNonDefaultValues["expire_passwords"]),
					resource.TestCheckResourceAttr(dataSourceName, "hard_expiry", testAccAccountPasswordPolicyDataSourceNonDefaultValues["hard_expiry"]),
					resource.TestCheckResourceAttr(dataSourceName, "max_password_age", testAccAccountPasswordPolicyDataSourceNonDefaultValues["max_password_age"]),
					resource.TestCheckResourceAttr(dataSourceName, "minimum_password_length", testAccAccountPasswordPolicyDataSourceNonDefaultValues["minimum_password_length"]),
					resource.TestCheckResourceAttr(dataSourceName, "password_reuse_prevention", testAccAccountPasswordPolicyDataSourceNonDefaultValues["password_reuse_prevention"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_lowercase_characters", testAccAccountPasswordPolicyDataSourceNonDefaultValues["require_lowercase_characters"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_numbers", testAccAccountPasswordPolicyDataSourceNonDefaultValues["require_numbers"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_symbols", testAccAccountPasswordPolicyDataSourceNonDefaultValues["require_symbols"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_uppercase_characters", testAccAccountPasswordPolicyDataSourceNonDefaultValues["require_uppercase_characters"]),
				),
			},
		},
	})
}

// TC-IAM-DAPP-02-Default Readback verifies DS-only reading without a resource: the fixed id,
// every field matches cloud GetAccountPasswordPolicy and equals service defaults
// (see testAccAccountPasswordPolicyDefaultValues).
// In the serial suite, basic runs first and its CheckDestroy restores defaults; when run alone, the account must already use defaults.
func testAccAccountPasswordPolicyDataSource_default(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_password_policy.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_default,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-password-policy"),
					resource.TestCheckResourceAttr(dataSourceName, "allow_users_to_change_password", testAccAccountPasswordPolicyDefaultValues["allow_users_to_change_password"]),
					resource.TestCheckResourceAttr(dataSourceName, "expire_passwords", testAccAccountPasswordPolicyDefaultValues["expire_passwords"]),
					resource.TestCheckResourceAttr(dataSourceName, "hard_expiry", testAccAccountPasswordPolicyDefaultValues["hard_expiry"]),
					resource.TestCheckResourceAttr(dataSourceName, "max_password_age", testAccAccountPasswordPolicyDefaultValues["max_password_age"]),
					resource.TestCheckResourceAttr(dataSourceName, "minimum_password_length", testAccAccountPasswordPolicyDefaultValues["minimum_password_length"]),
					resource.TestCheckResourceAttr(dataSourceName, "password_reuse_prevention", testAccAccountPasswordPolicyDefaultValues["password_reuse_prevention"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_lowercase_characters", testAccAccountPasswordPolicyDefaultValues["require_lowercase_characters"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_numbers", testAccAccountPasswordPolicyDefaultValues["require_numbers"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_symbols", testAccAccountPasswordPolicyDefaultValues["require_symbols"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_uppercase_characters", testAccAccountPasswordPolicyDefaultValues["require_uppercase_characters"]),
				),
			},
		},
	})
}

// TC-IAM-DAPP-03-Resource To DataSource Consistency verifies that after a full resource update,
// the DS immediately reads back the policy, matches the resource AttrPair and cloud state, and the second step explicitly asserts every updated expected value
// (each differs from the first step's NonDefault values); CheckDestroy confirms that the cloud policy returns to defaults.
func testAccAccountPasswordPolicyDataSource_consistency(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_password_policy.test"
	resourceName := "ctyun_iam_account_password_policy.test"

	pairChecks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-password-policy"),
		resource.TestCheckResourceAttrPair(dataSourceName, "allow_users_to_change_password", resourceName, "allow_users_to_change_password"),
		resource.TestCheckResourceAttrPair(dataSourceName, "hard_expiry", resourceName, "hard_expiry"),
		resource.TestCheckResourceAttrPair(dataSourceName, "max_password_age", resourceName, "max_password_age"),
		resource.TestCheckResourceAttrPair(dataSourceName, "minimum_password_length", resourceName, "minimum_password_length"),
		resource.TestCheckResourceAttrPair(dataSourceName, "password_reuse_prevention", resourceName, "password_reuse_prevention"),
		resource.TestCheckResourceAttrPair(dataSourceName, "require_lowercase_characters", resourceName, "require_lowercase_characters"),
		resource.TestCheckResourceAttrPair(dataSourceName, "require_numbers", resourceName, "require_numbers"),
		resource.TestCheckResourceAttrPair(dataSourceName, "require_symbols", resourceName, "require_symbols"),
		resource.TestCheckResourceAttrPair(dataSourceName, "require_uppercase_characters", resourceName, "require_uppercase_characters"),
		resource.TestCheckResourceAttrPair(dataSourceName, "expire_passwords", resourceName, "expire_passwords"),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountPasswordPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_basic,
				Check:  pairChecks,
			},
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_modified,
				Check: resource.ComposeAggregateTestCheckFunc(
					pairChecks,
					resource.TestCheckResourceAttr(dataSourceName, "allow_users_to_change_password", testAccAccountPasswordPolicyDataSourceModifiedValues["allow_users_to_change_password"]),
					resource.TestCheckResourceAttr(dataSourceName, "expire_passwords", testAccAccountPasswordPolicyDataSourceModifiedValues["expire_passwords"]),
					resource.TestCheckResourceAttr(dataSourceName, "hard_expiry", testAccAccountPasswordPolicyDataSourceModifiedValues["hard_expiry"]),
					resource.TestCheckResourceAttr(dataSourceName, "max_password_age", testAccAccountPasswordPolicyDataSourceModifiedValues["max_password_age"]),
					resource.TestCheckResourceAttr(dataSourceName, "minimum_password_length", testAccAccountPasswordPolicyDataSourceModifiedValues["minimum_password_length"]),
					resource.TestCheckResourceAttr(dataSourceName, "password_reuse_prevention", testAccAccountPasswordPolicyDataSourceModifiedValues["password_reuse_prevention"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_lowercase_characters", testAccAccountPasswordPolicyDataSourceModifiedValues["require_lowercase_characters"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_numbers", testAccAccountPasswordPolicyDataSourceModifiedValues["require_numbers"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_symbols", testAccAccountPasswordPolicyDataSourceModifiedValues["require_symbols"]),
					resource.TestCheckResourceAttr(dataSourceName, "require_uppercase_characters", testAccAccountPasswordPolicyDataSourceModifiedValues["require_uppercase_characters"]),
				),
			},
		},
	})
}

// TC-IAM-DAPP-04-Read Stability verifies two consecutive applies with identical configuration: each DS state
// matches cloud GetAccountPasswordPolicy; the second apply has no diff (PreApply / PostApplyPostRefresh);
// CheckDestroy confirms that the cloud policy returns to defaults after destruction.
func testAccAccountPasswordPolicyDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_password_policy.test"

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam-account-password-policy"),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccountPasswordPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_basic,
				Check:  checks,
			},
			{
				Config: testAccAccountPasswordPolicyDataSourceConfig_basic,
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

// testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud verifies DS state
// against cloud GetAccountPasswordPolicy without relying on managed resource state:
// it compares the fixed id and all policy fields (bool/int values are compared as state strings).
func testAccCheckAccountPasswordPolicyDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}
		if rs.Primary.ID != "iam-account-password-policy" {
			return fmt.Errorf("bad state id: got %s, want iam-account-password-policy", rs.Primary.ID)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountPasswordPolicy(ctx, conn)
		if err != nil {
			return fmt.Errorf("reading IAM Account Password Policy: %w", err)
		}
		if policy == nil {
			return fmt.Errorf("IAM Account Password Policy not found in cloud")
		}

		cloudValues := map[string]string{
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
		for attribute, want := range cloudValues {
			if got := rs.Primary.Attributes[attribute]; got != want {
				return fmt.Errorf("bad %s: state=%q GetAccountPasswordPolicy=%q", attribute, got, want)
			}
		}

		return nil
	}
}

// All values differ from testAccAccountPasswordPolicyDefaultValues (expire_passwords is derived as true when max_password_age > 0).
// Used only by DS ACC; it does not modify the resource-side shared Config_basic.
var testAccAccountPasswordPolicyDataSourceNonDefaultValues = map[string]string{
	"allow_users_to_change_password": "false",
	"expire_passwords":               "true",
	"hard_expiry":                    "true",
	"max_password_age":               "90",
	"minimum_password_length":        "16",
	"password_reuse_prevention":      "5",
	"require_lowercase_characters":   "false",
	"require_numbers":                "false",
	"require_symbols":                "true",
	"require_uppercase_characters":   "true",
}

const testAccAccountPasswordPolicyDataSourceConfig_nonDefaultResource = `
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = false
  hard_expiry                    = true
  max_password_age               = 90
  minimum_password_length        = 16
  password_reuse_prevention      = 5
  require_lowercase_characters   = false
  require_numbers                = false
  require_symbols                = true
  require_uppercase_characters   = true
}
`

var testAccAccountPasswordPolicyDataSourceConfig_basic = acctest.ConfigCompose(
	testAccAccountPasswordPolicyDataSourceConfig_nonDefaultResource,
	`
data "ctyun_iam_account_password_policy" "test" {
  depends_on = [ctyun_iam_account_password_policy.test]
}
`,
)

// Rewrites every value relative to NonDefault (including expire_passwords: max_password_age 90 -> 0 changes true -> false).
// Used only by DS ACC; it does not modify the resource-side shared Config_modified.
var testAccAccountPasswordPolicyDataSourceModifiedValues = map[string]string{
	"allow_users_to_change_password": "true",
	"expire_passwords":               "false",
	"hard_expiry":                    "false",
	"max_password_age":               "0",
	"minimum_password_length":        "9",
	"password_reuse_prevention":      "3",
	"require_lowercase_characters":   "true",
	"require_numbers":                "true",
	"require_symbols":                "false",
	"require_uppercase_characters":   "false",
}

const testAccAccountPasswordPolicyDataSourceConfig_modifiedResource = `
resource "ctyun_iam_account_password_policy" "test" {
  allow_users_to_change_password = true
  hard_expiry                    = false
  max_password_age               = 0
  minimum_password_length        = 9
  password_reuse_prevention      = 3
  require_lowercase_characters   = true
  require_numbers                = true
  require_symbols                = false
  require_uppercase_characters   = false
}
`

var testAccAccountPasswordPolicyDataSourceConfig_modified = acctest.ConfigCompose(
	testAccAccountPasswordPolicyDataSourceConfig_modifiedResource,
	`
data "ctyun_iam_account_password_policy" "test" {
  depends_on = [ctyun_iam_account_password_policy.test]
}
`,
)

const testAccAccountPasswordPolicyDataSourceConfig_default = `
data "ctyun_iam_account_password_policy" "test" {}
`
