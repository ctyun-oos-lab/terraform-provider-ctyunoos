// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMAccountSummaryDataSource_schema verifies the ctyun_iam_account_summary data source
// schema: it has no Required inputs, and all 18 summary fields are Computed TypeInt values.
func TestUnitIAMAccountSummaryDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceAccountSummary()

	for attrName := range tfiam.AccountSummaryAttributeKeys() {
		attr, ok := dataSource.Schema[attrName]
		if !ok {
			t.Fatalf("expected %s schema", attrName)
		}
		if !attr.Computed {
			t.Fatalf("expected %s Computed", attrName)
		}
		if attr.Required {
			t.Fatalf("expected %s not Required", attrName)
		}
		if attr.Type != schema.TypeInt {
			t.Fatalf("expected %s TypeInt, got %v", attrName, attr.Type)
		}
	}
}

// TestUnitIAMAccountSummaryDataSource_attributeKeys verifies the 18 mappings from Terraform attribute names to
// GetAccountSummary SummaryMap keys.
func TestUnitIAMAccountSummaryDataSource_attributeKeys(t *testing.T) {
	t.Parallel()

	keys := tfiam.AccountSummaryAttributeKeys()

	if got, want := len(keys), 18; got != want {
		t.Fatalf("expected %d account summary attributes, got %d", want, got)
	}

	testCases := map[string]string{
		"accesskeys_per_user_quota":         "AccessKeysPerUserQuota",
		"account_accesskeys_present":        "AccountAccessKeysPresent",
		"attached_policies_per_group_quota": "AttachedPoliciesPerGroupQuota",
		"attached_policies_per_role_quota":  "AttachedPoliciesPerRoleQuota",
		"attached_policies_per_user_quota":  "AttachedPoliciesPerUserQuota",
		"groups":                            "Groups",
		"groups_per_user_quota":             "GroupsPerUserQuota",
		"groups_quota":                      "GroupsQuota",
		"mfa_devices":                       "MFADevices",
		"mfa_devices_inuse":                 "MFADevicesInUse",
		"policies":                          "Policies",
		"policies_quota":                    "PoliciesQuota",
		"providers":                         "Providers",
		"providers_quota":                   "ProvidersQuota",
		"roles":                             "Roles",
		"roles_quota":                       "RolesQuota",
		"users":                             "Users",
		"users_quota":                       "UsersQuota",
	}

	for attrName, want := range testCases {
		if got := keys[attrName]; got != want {
			t.Fatalf("expected %s to map to %q, got %q", attrName, want, got)
		}
	}
}

// TestUnitIAMAccountSummaryDataSource_value verifies AccountSummaryValue:
// an existing key returns its value; a missing key falls back to 0 (covering the former TC-IAM-DAS-03 Missing Key Fallback).
func TestUnitIAMAccountSummaryDataSource_value(t *testing.T) {
	t.Parallel()

	summary := map[string]int32{
		"Users":      10,
		"UsersQuota": 500,
	}

	if got, want := tfiam.AccountSummaryValue(summary, "Users"), 10; got != want {
		t.Fatalf("expected users %d, got %d", want, got)
	}

	if got, want := tfiam.AccountSummaryValue(summary, "Missing"), 0; got != want {
		t.Fatalf("expected missing key value %d, got %d", want, got)
	}
}

// TC-IAM-DAS-01-Basic Summary Read verifies account-summary reading: fixed id=iam_account_summary,
// all 18 count fields match the cloud GetAccountSummary SummaryMap (missing keys are treated as 0, as in the DS);
// key fields are cross-checked with hard-coded mappings to avoid missing errors through a mapping function shared with the DS.
// providers_quota is an OOS extension (the SDK has no SummaryKeyTypeProvidersQuota): assert that SummaryMap
// contains the ProvidersQuota key and use TestCheckResourceAttrSet to confirm the DS wrote the field.
// Serial execution is required because the full-account counts are compared; if run with pagination tests,
// the snapshot read during Apply can differ from GetAccountSummary during Check due to resources created by other tests (for example, state=28 vs Users=129).
func TestAccIAMAccountSummaryDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_account_summary.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountSummaryDataSourceConfig_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountSummaryDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam_account_summary"),
					resource.TestCheckResourceAttrSet(dataSourceName, "providers_quota"),
				),
			},
		},
	})
}

// TC-IAM-DAS-02-Counter Consistency verifies that usage counts change in the correct direction as resources are added and removed:
// 1) DS-only baseline read (users/groups/policies/roles);
// 2) create one user/group/policy/role each and read again; all four counts should be baseline +1;
// both steps match cloud GetAccountSummary. These account-level shared counts require a serial Test.
func TestAccIAMAccountSummaryDataSource_counterConsistency(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_account_summary.test"
	var baseline testAccAccountSummaryCounters

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckUserDestroy(ctx, t),
			testAccCheckGroupDestroy(ctx, t),
			testAccCheckPolicyDestroy(ctx, t),
			testAccCheckRoleDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccAccountSummaryDataSourceConfig_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountSummaryDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam_account_summary"),
					testAccCheckAccountSummaryCaptureCounters(dataSourceName, &baseline),
				),
			},
			{
				Config: testAccAccountSummaryDataSourceConfig_withResources(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccountSummaryDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, "iam_account_summary"),
					testAccCheckAccountSummaryCountersIncreasedBy(dataSourceName, &baseline, 1),
				),
			},
		},
	})
}

// testAccCheckAccountSummaryDataSourceStateMatchesCloud verifies DS state against cloud
// GetAccountSummary: it compares the fixed id and 18 attributes obtained from SummaryMap through AccountSummaryAttributeKeys
// (same as DS Read: missing keys become 0).
// Key fields are also cross-checked with hard-coded mappings so an error shared through accountSummaryAttributeKeys()
// is not amplified and missed by the comparison. The OOS ProvidersQuota extension has a separate SummaryMap-presence assertion.
func testAccCheckAccountSummaryDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}
		if rs.Primary.ID != "iam_account_summary" {
			return fmt.Errorf("bad state id: got %s, want iam_account_summary", rs.Primary.ID)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
		if err != nil {
			return fmt.Errorf("reading IAM Account Summary: %w", err)
		}
		if output == nil || output.SummaryMap == nil {
			return fmt.Errorf("IAM Account Summary SummaryMap is empty")
		}

		// OOS extension key: the AWS SDK has no SummaryKeyTypeProvidersQuota. When missing,
		// AccountSummaryValue falls back to 0 and DS state is also 0, so comparison cannot detect a key mismatch.
		if _, ok := output.SummaryMap["ProvidersQuota"]; !ok {
			return fmt.Errorf("GetAccountSummary SummaryMap missing key ProvidersQuota: %#v", output.SummaryMap)
		}

		for attrName, summaryKey := range tfiam.AccountSummaryAttributeKeys() {
			want := strconv.Itoa(tfiam.AccountSummaryValue(output.SummaryMap, summaryKey))
			if got := rs.Primary.Attributes[attrName]; got != want {
				return fmt.Errorf("bad %s: state=%q GetAccountSummary[%s]=%q", attrName, got, summaryKey, want)
			}
		}

		// Cross-check with a hard-coded mapping independent of the DS accountSummaryAttributeKeys().
		crossCheck := map[string]string{
			"users":           "Users",
			"groups":          "Groups",
			"policies":        "Policies",
			"roles":           "Roles",
			"providers_quota": "ProvidersQuota",
		}
		for attrName, summaryKey := range crossCheck {
			want := strconv.Itoa(tfiam.AccountSummaryValue(output.SummaryMap, summaryKey))
			if got := rs.Primary.Attributes[attrName]; got != want {
				return fmt.Errorf("cross-check bad %s: state=%q GetAccountSummary[%s]=%q", attrName, got, summaryKey, want)
			}
		}

		return nil
	}
}

type testAccAccountSummaryCounters struct {
	users    int
	groups   int
	policies int
	roles    int
}

func testAccCheckAccountSummaryCaptureCounters(dataSourceName string, out *testAccAccountSummaryCounters) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		counters, err := testAccAccountSummaryCountersFromState(s, dataSourceName)
		if err != nil {
			return err
		}
		*out = counters
		return nil
	}
}

func testAccCheckAccountSummaryCountersIncreasedBy(dataSourceName string, baseline *testAccAccountSummaryCounters, delta int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := testAccAccountSummaryCountersFromState(s, dataSourceName)
		if err != nil {
			return err
		}

		checks := []struct {
			name     string
			got      int
			baseline int
		}{
			{"users", got.users, baseline.users},
			{"groups", got.groups, baseline.groups},
			{"policies", got.policies, baseline.policies},
			{"roles", got.roles, baseline.roles},
		}
		for _, c := range checks {
			want := c.baseline + delta
			if c.got != want {
				return fmt.Errorf("%s = %d, want baseline %d + %d = %d", c.name, c.got, c.baseline, delta, want)
			}
		}
		return nil
	}
}

func testAccAccountSummaryCountersFromState(s *terraform.State, dataSourceName string) (testAccAccountSummaryCounters, error) {
	var out testAccAccountSummaryCounters

	rs, ok := s.RootModule().Resources[dataSourceName]
	if !ok {
		return out, fmt.Errorf("Not found: %s", dataSourceName)
	}

	parse := func(attr string) (int, error) {
		v, err := strconv.Atoi(rs.Primary.Attributes[attr])
		if err != nil {
			return 0, fmt.Errorf("invalid %s value: %w", attr, err)
		}
		return v, nil
	}

	var err error
	if out.users, err = parse("users"); err != nil {
		return out, err
	}
	if out.groups, err = parse("groups"); err != nil {
		return out, err
	}
	if out.policies, err = parse("policies"); err != nil {
		return out, err
	}
	if out.roles, err = parse("roles"); err != nil {
		return out, err
	}
	return out, nil
}

const testAccAccountSummaryDataSourceConfig_basic = `
data "ctyun_iam_account_summary" "test" {}
`

func testAccAccountSummaryDataSourceConfig_withResources(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = "%[1]s-user"
}

resource "ctyun_iam_group" "test" {
  name = "%[1]s-group"
}

resource "ctyun_iam_policy" "test" {
  name = "%[1]s-policy"

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

resource "ctyun_iam_role" "test" {
  name               = "%[1]s-role"
  assume_role_policy = %[2]s
}

data "ctyun_iam_account_summary" "test" {
  depends_on = [
    ctyun_iam_user.test,
    ctyun_iam_group.test,
    ctyun_iam_policy.test,
    ctyun_iam_role.test,
  ]
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}
