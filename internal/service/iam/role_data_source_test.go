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
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMRoleDataSource_schema verifies the schema shape of the ctyun_iam_role data source:
// name is Required (not Optional); arn/assume_role_policy/create_date/description/
// max_session_duration/unique_id are all Computed; upstream fields such as path/permissions_boundary/tags are absent.
func TestUnitIAMRoleDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceRole()

	nameSchema := dataSource.Schema[names.AttrName]
	if nameSchema == nil {
		t.Fatal("expected name schema")
	}
	if !nameSchema.Required || nameSchema.Optional {
		t.Fatalf("expected name to be Required (not Optional), got Required=%v Optional=%v", nameSchema.Required, nameSchema.Optional)
	}

	for _, name := range []string{
		names.AttrARN,
		"assume_role_policy",
		"create_date",
		names.AttrDescription,
		"max_session_duration",
		"unique_id",
	} {
		s, ok := dataSource.Schema[name]
		if !ok {
			t.Fatalf("expected %s schema", name)
		}
		if !s.Computed || s.Optional || s.Required {
			t.Fatalf("expected %s to be Computed only, got Computed=%v Optional=%v Required=%v", name, s.Computed, s.Optional, s.Required)
		}
	}

	if got := dataSource.Schema["max_session_duration"].Type; got != schema.TypeInt {
		t.Fatalf("expected max_session_duration to be TypeInt, got %v", got)
	}

	for _, name := range []string{
		names.AttrPath,
		"permissions_boundary",
		names.AttrTags,
		names.AttrTagsAll,
		"force_detach_policies",
		"inline_policy",
		"managed_policy_arns",
	} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("unexpected %s schema", name)
		}
	}
}

// TestUnitIAMRoleDataSource_nameValidation verifies the ValidateFunc of the ctyun_iam_role data source
// input name (covers the former TC-IAM-DROLE-03 Name Validation Matrix):
// valid special characters and the upper length bound of 64 pass; empty values, 65 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMRoleDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceRole()

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_special_chars", value: "Role_-,.+@=123", errContains: nil},
		{name: "valid_max_length", value: strings.Repeat("a", 64), errContains: nil},
		{name: "empty", value: "", errContains: []string{
			"expected length of name to be in the range (1 - 64)",
		}},
		{name: "too_long", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of name to be in the range (1 - 64)",
		}},
		{name: "invalid_space", value: "t est", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},
		{name: "invalid_exclamation", value: "role!", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", errContains: []string{
			"invalid value for name",
			`must match [\w+=,.@-]`,
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validateFunc := dataSource.Schema[names.AttrName].ValidateFunc
			if validateFunc == nil {
				t.Fatal("expected name ValidateFunc")
			}

			_, errs := validateFunc(tc.value, names.AttrName)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for name=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for name=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for name missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TC-IAM-DROLE-01-Basic Lookup verifies reading by name after creating a role:
// the DS state fields id/arn/name/description/max_session_duration/create_date/unique_id match the cloud GetRole result
// (not compared against managed resource state; CheckDestroy at the end confirms the fixture role is destroyed).
func TestAccIAMRoleDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleDataSourceConfig_basic(roleName),
				Check:  testAccCheckRoleDataSourceStateMatchesCloud(ctx, t, dataSourceName),
			},
		},
	})
}

// TC-IAM-DROLE-02-AssumeRolePolicy Decode verifies that assume_role_policy is URL-decoded after reading the role:
// the DS state is compared independently against the cloud GetRole result (assume_role_policy is QueryUnescaped and
// compared for semantic equivalence); it is also AttrPair-matched against the fixture role and compared for semantic
// equivalence with the expected JSON (EquivalentJSON); CheckDestroy at the end.
func TestAccIAMRoleDataSource_assumeRolePolicy(t *testing.T) {
	ctx := acctest.Context(t)
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_role.test"
	resourceName := "ctyun_iam_role.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleDataSourceConfig_assumeRolePolicy(roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttrPair(dataSourceName, "assume_role_policy", resourceName, "assume_role_policy"),
					acctest.CheckResourceAttrEquivalentJSON(dataSourceName, "assume_role_policy", testAccRoleAssumePolicy("ec2.amazonaws.com")),
				),
			},
		},
	})
}

// TC-IAM-DROLE-04-Missing Role Negative verifies that reading a non-existent role name returns a clear NotFound-style error,
// and that no data.ctyun_iam_role remains in state (no half-written result).
func TestAccIAMRoleDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRoleDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccRoleDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckRoleDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DROLE-04-Other Account Negative verifies that reading a role that exists under another account cannot be found
// by name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_role left in state (no half-written result).
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the role with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMRoleDataSource_otherAccount(t *testing.T) {
	crossAccountRole := testAccCrossAccountRoleName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRoleDataSourceConfig_otherAccountRole(crossAccountRole),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccRoleDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckRoleDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DROLE-05-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud GetRole result; the second read-back shows no drift relative to the first snapshot for
// id/arn/name/description/max_session_duration/create_date/unique_id/assume_role_policy (assume_role_policy is compared
// for semantic equivalence); CheckDestroy at the end.
func TestAccIAMRoleDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	roleName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_role.test"
	var snapshot testAccRoleDataSourceSnapshot

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleDataSourceConfig_basic(roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckRoleDataSourceCaptureSnapshot(dataSourceName, &snapshot),
				),
			},
			{
				Config: testAccRoleDataSourceConfig_basic(roleName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRoleDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckRoleDataSourceMatchesSnapshot(dataSourceName, &snapshot),
				),
			},
		},
	})
}

// testAccCheckRoleDataSourceStateMatchesCloud checks that the data source state matches the cloud GetRole result without
// relying on managed resource state: the cloud role is fetched by the DS id (role name), comparing
// id/arn/name/description/max_session_duration/create_date/unique_id/assume_role_policy.
func testAccCheckRoleDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		role, err := tfiam.FindRoleByName(ctx, conn, rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("reading IAM Role (%s): %w", rs.Primary.ID, err)
		}
		if role == nil {
			return fmt.Errorf("IAM Role (%s) not found in cloud", rs.Primary.ID)
		}

		wantName := aws.ToString(role.RoleName)
		if rs.Primary.ID != wantName {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, wantName)
		}
		if got := rs.Primary.Attributes[names.AttrName]; got != wantName {
			return fmt.Errorf("bad name: got %s, want %s", got, wantName)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], aws.ToString(role.Arn); got != want {
			return fmt.Errorf("bad arn: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrDescription], aws.ToString(role.Description); got != want {
			return fmt.Errorf("bad description: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["max_session_duration"], strconv.Itoa(int(aws.ToInt32(role.MaxSessionDuration))); got != want {
			return fmt.Errorf("bad max_session_duration: got %s, want %s", got, want)
		}
		if role.CreateDate == nil {
			if got := rs.Primary.Attributes["create_date"]; got != "" {
				return fmt.Errorf("bad create_date: got %s, want empty (CreateDate is nil in cloud)", got)
			}
		} else if got, want := rs.Primary.Attributes["create_date"], aws.ToTime(role.CreateDate).Format(time.RFC3339); got != want {
			return fmt.Errorf("bad create_date: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["unique_id"], aws.ToString(role.RoleId); got != want {
			return fmt.Errorf("bad unique_id: got %s, want %s", got, want)
		}

		cloudPolicy, err := url.QueryUnescape(aws.ToString(role.AssumeRolePolicyDocument))
		if err != nil {
			return fmt.Errorf("parsing IAM Role (%s) assume_role_policy: %w", wantName, err)
		}
		statePolicy := rs.Primary.Attributes["assume_role_policy"]
		if !verify.PolicyStringsEquivalent(statePolicy, cloudPolicy) {
			return fmt.Errorf("bad assume_role_policy: got %s, want %s", statePolicy, cloudPolicy)
		}

		return nil
	}
}

// testAccRoleDataSourceSnapshot saves the fields read back by the DS for drift comparison across two consecutive applies.
type testAccRoleDataSourceSnapshot struct {
	id                 string
	arn                string
	name               string
	description        string
	maxSessionDuration string
	createDate         string
	uniqueID           string
	assumeRolePolicy   string
}

func testAccCheckRoleDataSourceCaptureSnapshot(dataSourceName string, snapshot *testAccRoleDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		*snapshot = testAccRoleDataSourceSnapshot{
			id:                 rs.Primary.ID,
			arn:                rs.Primary.Attributes[names.AttrARN],
			name:               rs.Primary.Attributes[names.AttrName],
			description:        rs.Primary.Attributes[names.AttrDescription],
			maxSessionDuration: rs.Primary.Attributes["max_session_duration"],
			createDate:         rs.Primary.Attributes["create_date"],
			uniqueID:           rs.Primary.Attributes["unique_id"],
			assumeRolePolicy:   rs.Primary.Attributes["assume_role_policy"],
		}
		return nil
	}
}

func testAccCheckRoleDataSourceMatchesSnapshot(dataSourceName string, snapshot *testAccRoleDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if snapshot == nil || snapshot.id == "" {
			return fmt.Errorf("role data source snapshot is empty")
		}

		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		if got, want := rs.Primary.ID, snapshot.id; got != want {
			return fmt.Errorf("drift id: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], snapshot.arn; got != want {
			return fmt.Errorf("drift arn: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrName], snapshot.name; got != want {
			return fmt.Errorf("drift name: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes[names.AttrDescription], snapshot.description; got != want {
			return fmt.Errorf("drift description: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["max_session_duration"], snapshot.maxSessionDuration; got != want {
			return fmt.Errorf("drift max_session_duration: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["create_date"], snapshot.createDate; got != want {
			return fmt.Errorf("drift create_date: got %s, want %s", got, want)
		}
		if got, want := rs.Primary.Attributes["unique_id"], snapshot.uniqueID; got != want {
			return fmt.Errorf("drift unique_id: got %s, want %s", got, want)
		}
		gotPolicy := rs.Primary.Attributes["assume_role_policy"]
		if !verify.PolicyStringsEquivalent(gotPolicy, snapshot.assumeRolePolicy) {
			return fmt.Errorf("drift assume_role_policy: got %s, want %s", gotPolicy, snapshot.assumeRolePolicy)
		}

		return nil
	}
}

func testAccRoleDataSourceConfig_basic(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name                 = %[1]q
  description          = "test role"
  max_session_duration = 3600
  assume_role_policy   = %[2]s
}

data "ctyun_iam_role" "test" {
  name = ctyun_iam_role.test.name
}
`, roleName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleDataSourceConfig_assumeRolePolicy(roleName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}

data "ctyun_iam_role" "test" {
  name = ctyun_iam_role.test.name
}
`, roleName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRoleDataSourceConfig_notFound(roleName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_role" "test" {
  name = %[1]q
}
`, roleName)
}

func testAccRoleDataSourceConfig_otherAccountRole(roleName string) string {
	return fmt.Sprintf(`
# The cross-account role is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_role" "test" {
  name = %[1]q
}
`, roleName)
}

func testAccRoleDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed role data source read left no residual state
`
}

func testAccCheckRoleDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_role.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q name=%q arn=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrName],
				rs.Primary.Attributes[names.AttrARN],
			)
		}
		return nil
	}
}
