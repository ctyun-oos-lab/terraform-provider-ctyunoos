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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMRolesDataSource_schema verifies the schema shape of ctyun_iam_roles:
// it contains arns/names/name_regex/roles; it does not contain the upstream single-resource name;
// the roles elements contain arn/assume_role_policy/create_date/description/max_session_duration/name/unique_id;
// name_regex has a ValidateFunc.
func TestUnitIAMRolesDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceRoles()

	for _, name := range []string{names.AttrARNs, names.AttrNames, "name_regex", "roles"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	if _, ok := dataSource.Schema[names.AttrName]; ok {
		t.Fatalf("unexpected name schema")
	}

	roleSchema := dataSource.Schema["roles"].Elem.(*schema.Resource).Schema
	for _, name := range []string{
		names.AttrARN,
		"assume_role_policy",
		"create_date",
		names.AttrDescription,
		"max_session_duration",
		names.AttrName,
		"unique_id",
	} {
		if _, ok := roleSchema[name]; !ok {
			t.Fatalf("expected roles.%s schema", name)
		}
	}

	if dataSource.Schema["name_regex"].ValidateFunc == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}
}

// TestUnitIAMRolesDataSource_nameRegexValidation verifies the schema ValidateFunc of name_regex:
// valid regular expressions pass; various invalid regex forms return clear parsing errors (covers the former TC-IAM-DROLES-04).
func TestUnitIAMRolesDataSource_nameRegexValidation(t *testing.T) {
	t.Parallel()

	validate := tfiam.DataSourceRoles().Schema["name_regex"].ValidateFunc
	if validate == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_prefix", value: "^tf-", errContains: nil},
		{name: "valid_exact", value: `^my-role-123$`, errContains: nil},
		{name: "valid_empty", value: "", errContains: nil},

		{name: "missing_closing_bracket", value: "[", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing closing ]",
		}},
		{name: "missing_closing_paren", value: "(", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing closing )",
		}},
		{name: "trailing_backslash", value: `\`, errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"trailing backslash",
		}},
		{name: "repetition_star_no_argument", value: "*", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing argument to repetition operator",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validate(tc.value, "name_regex")
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for name_regex=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for name_regex=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for name_regex missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TestUnitIAMRolesDataSource_flattenRoles verifies flattenRoles:
// the normal path maps arn/assume_role_policy (URL-decoded)/create_date/description/
// max_session_duration/name/unique_id; empty input returns a non-nil empty slice; when CreateDate is nil, create_date is "".
func TestUnitIAMRolesDataSource_flattenRoles(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		assumeRolePolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"sts:AssumeRole","Principal":{"Service":"ec2.amazonaws.com"}}]}`
		roles := []awstypes.Role{
			{
				Arn:                      aws.String("arn:ctyun:iam::969618309127:role/example-role"),
				AssumeRolePolicyDocument: aws.String(url.QueryEscape(assumeRolePolicy)),
				CreateDate:               aws.Time(createDate),
				Description:              aws.String("example description"),
				MaxSessionDuration:       aws.Int32(3600),
				RoleId:                   aws.String("AROA6DQOBIAD7BL4YN5OA"),
				RoleName:                 aws.String("example-role"),
			},
		}

		tfRoles := tfiam.FlattenRoles(roles)

		if got, want := len(tfRoles), 1; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}

		if got, want := tfRoles[0][names.AttrARN], "arn:ctyun:iam::969618309127:role/example-role"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}

		if got, want := tfRoles[0]["assume_role_policy"], assumeRolePolicy; got != want {
			t.Fatalf("expected assume_role_policy %q, got %q", want, got)
		}

		if got, want := tfRoles[0]["create_date"], "2026-06-09T08:30:10Z"; got != want {
			t.Fatalf("expected create_date %q, got %q", want, got)
		}

		if got, want := tfRoles[0][names.AttrDescription], "example description"; got != want {
			t.Fatalf("expected description %q, got %q", want, got)
		}

		if got, want := tfRoles[0]["max_session_duration"], 3600; got != want {
			t.Fatalf("expected max_session_duration %v, got %v", want, got)
		}

		if got, want := tfRoles[0][names.AttrName], "example-role"; got != want {
			t.Fatalf("expected name %q, got %q", want, got)
		}

		if got, want := tfRoles[0]["unique_id"], "AROA6DQOBIAD7BL4YN5OA"; got != want {
			t.Fatalf("expected unique_id %q, got %q", want, got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		tfRoles := tfiam.FlattenRoles(nil)
		if tfRoles == nil {
			t.Fatal("expected non-nil empty roles slice for nil input")
		}
		if got, want := len(tfRoles), 0; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}

		tfRoles = tfiam.FlattenRoles([]awstypes.Role{})
		if tfRoles == nil {
			t.Fatal("expected non-nil empty roles slice for empty input")
		}
		if got, want := len(tfRoles), 0; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
	})

	t.Run("nil_create_date", func(t *testing.T) {
		t.Parallel()

		tfRoles := tfiam.FlattenRoles([]awstypes.Role{
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:role/nil-date"),
				RoleId:     aws.String("AROANILCREATEDATE"),
				RoleName:   aws.String("nil-date"),
				CreateDate: nil,
			},
		})
		if got, want := len(tfRoles), 1; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		if got, want := tfRoles[0]["create_date"], ""; got != want {
			t.Fatalf("expected create_date %q, got %q", want, got)
		}
	})
}

// TestUnitIAMRolesDataSource_filterRolesByNameRegex verifies local name_regex filtering:
// partial match, full match (output = input), no match at all (nil empty slice), and empty input.
func TestUnitIAMRolesDataSource_filterRolesByNameRegex(t *testing.T) {
	t.Parallel()

	roles := []awstypes.Role{
		{RoleName: aws.String("1111")},
		{RoleName: aws.String("123byrole")},
		{RoleName: aws.String("Terraform-1")},
	}

	t.Run("partial_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterRolesByNameRegex(roles, regexp.MustCompile(`^123`))
		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		if got, want := aws.ToString(output[0].RoleName), "123byrole"; got != want {
			t.Fatalf("expected role name %q, got %q", want, got)
		}
	})

	t.Run("all_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterRolesByNameRegex(roles, regexp.MustCompile(`.*`))
		if got, want := len(output), len(roles); got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		for i := range roles {
			if got, want := aws.ToString(output[i].RoleName), aws.ToString(roles[i].RoleName); got != want {
				t.Fatalf("expected role_name[%d] %q, got %q", i, want, got)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterRolesByNameRegex(roles, regexp.MustCompile(`^nomatch-`))
		if output != nil {
			t.Fatalf("expected nil slice for no match, got %#v", output)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterRolesByNameRegex(nil, regexp.MustCompile(`.*`))
		if output != nil {
			t.Fatalf("expected nil slice for nil input, got %#v", output)
		}
		output = tfiam.FilterRolesByNameRegex([]awstypes.Role{}, regexp.MustCompile(`.*`))
		if output != nil {
			t.Fatalf("expected nil slice for empty input, got %#v", output)
		}
	})
}

// TC-IAM-DROLES-01-Basic List:
// Sub-scenario 1: create only 1 role without name_regex; id is the Region, and arns/names/roles match the full ListRoles
// result exactly (no more, no less), with each role's fields matching ListRoles.
// Sub-scenario 2: create multiple roles; same checks as sub-scenario 1. CheckDestroy at the end.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account; if run in parallel with
// pagination etc., the snapshot read at Apply would differ from the List count at Check time because of fixtures created by other tests.
func TestAccIAMRolesDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_roles.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig_basicSingle(rName),
				Check:  testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, ""),
			},
			{
				Config: testAccRolesDataSourceConfig_basicMultiple(rName),
				Check:  testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, ""),
			},
		},
	})
}

// TC-IAM-DROLES-02-NameRegex Filter verifies creating two roles and using name_regex to match exactly one of them:
// ExactMatch compares against "ListRoles + local filtering with the same rule"; it also explicitly asserts that the
// roles/names/arns counts are all 1 and that names contains the expected roleA; CheckDestroy at the end.
func TestAccIAMRolesDataSource_nameRegex(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	roleA := rName + "-a"
	roleB := rName + "-b"
	dataSourceName := "data.ctyun_iam_roles.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(roleA))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig_nameRegex(roleA, roleB),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", roleA),
				),
			},
		},
	})
}

// TC-IAM-DROLES-05-Empty Result verifies: first create at least 1 role (ensuring ListRoles is non-empty),
// then read with a name_regex that cannot match anything; ExactMatch compares against the filtered ListRoles result and
// explicitly asserts that roles/arns/names are all empty (due to regex filtering, not because the cloud has no roles),
// with id still being the Region; CheckDestroy at the end.
func TestAccIAMRolesDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_roles.test"
	// A fixed regex unrelated to the fixture role names, ensuring the filtered result is empty.
	nameRegex := "^tf-no-such-role-zzzzzzzz$"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "0"),
				),
			},
		},
	})
}

// ListRoles documentation: MaxItems defaults to 100; beyond that IsTruncated=true and a Marker is required to continue.
// The DS Read cannot change MaxItems, so this case creates more data than the default page size to force the real Read through multi-page aggregation.
const testAccIAMRolesListDefaultMaxItems = 100

// TC-IAM-DROLES-06-Pagination verifies that when the total number of roles in the account exceeds the ListRoles default
// MaxItems, the data source Read (empty ListRolesInput → default pagination) still returns all roles matching name_regex.
func TestAccIAMRolesDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	roleCount := testAccIAMRolesListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_roles.test"
	nameRegex := fmt.Sprintf("^%s-", regexp.QuoteMeta(rName))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckRolesPaginationQuota(ctx, t, roleCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig_pagination(rName, roleCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckRolesListDefaultPageTruncated(ctx, t),
					testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", strconv.Itoa(roleCount)),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", strconv.Itoa(roleCount)),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", strconv.Itoa(roleCount)),
				),
			},
		},
	})
}

// TC-IAM-DROLES-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state is
// ExactMatch-compared against the cloud ListRoles (filtered with the same name_regex); the second apply shows no diff
// (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused by unstable read-back formats of flattenRoles /
// create_date etc. name_regex is used to bound the result set so the test can run in parallel.
func TestAccIAMRolesDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	roleA := rName + "-a"
	roleB := rName + "-b"
	dataSourceName := "data.ctyun_iam_roles.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(roleA))

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckRolesDataSourceExactMatchListRoles(ctx, t, dataSourceName, nameRegex),
		resource.TestCheckResourceAttr(dataSourceName, "roles.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
		resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", roleA),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckRoleDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccRolesDataSourceConfig_nameRegex(roleA, roleB),
				Check:  checks,
			},
			{
				Config: testAccRolesDataSourceConfig_nameRegex(roleA, roleB),
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

func testAccCheckRolesDataSourceExactMatchListRoles(ctx context.Context, t *testing.T, dataSourceName, nameRegex string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		meta := acctest.ProviderMeta(ctx, t)
		wantID := meta.EffectiveRealRegion(conns.NewResourceContext(ctx, names.IAM, "", "", ""))
		if wantID == "" {
			return fmt.Errorf("could not resolve IAM EffectiveRealRegion for id check")
		}
		if got := rs.Primary.ID; got != wantID {
			return fmt.Errorf("bad id: got %s, want %s (region)", got, wantID)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["roles.#"])
		if err != nil {
			return fmt.Errorf("parsing roles.#: %w", err)
		}

		conn := meta.IAMClient(ctx)
		cloudRoles, err := tfiam.FindRoles(ctx, conn, &iam.ListRolesInput{})
		if err != nil {
			return fmt.Errorf("listing IAM Roles: %w", err)
		}
		if nameRegex != "" {
			r, err := regexp.Compile(nameRegex)
			if err != nil {
				return fmt.Errorf("compiling name_regex (%s): %w", nameRegex, err)
			}
			cloudRoles = tfiam.FilterRolesByNameRegex(cloudRoles, r)
		}

		if stateCount != len(cloudRoles) {
			return fmt.Errorf("roles count mismatch: state=%d listRoles=%d", stateCount, len(cloudRoles))
		}

		cloudByARN := make(map[string]awstypes.Role, len(cloudRoles))
		cloudARNs := make(map[string]struct{}, len(cloudRoles))
		cloudNames := make(map[string]struct{}, len(cloudRoles))
		for _, role := range cloudRoles {
			arn := aws.ToString(role.Arn)
			cloudByARN[arn] = role
			cloudARNs[arn] = struct{}{}
			cloudNames[aws.ToString(role.RoleName)] = struct{}{}
		}

		stateARNs := collectTypeSetValues(attrs, names.AttrARNs)
		stateNames := collectTypeSetValues(attrs, names.AttrNames)
		if err := setExactMatch(names.AttrARNs, stateARNs, cloudARNs); err != nil {
			return err
		}
		if err := setExactMatch(names.AttrNames, stateNames, cloudNames); err != nil {
			return err
		}

		seenARN := make(map[string]struct{}, stateCount)
		for i := 0; i < stateCount; i++ {
			prefix := fmt.Sprintf("roles.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty roles.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate roles.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("roles.%d.arn %q not found in ListRoles", i, stateARN)
			}

			if got, want := attrs[prefix+names.AttrName], aws.ToString(cloud.RoleName); got != want {
				return fmt.Errorf("bad roles.%d.name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+names.AttrDescription], aws.ToString(cloud.Description); got != want {
				return fmt.Errorf("bad roles.%d.description: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"max_session_duration"], strconv.Itoa(int(aws.ToInt32(cloud.MaxSessionDuration))); got != want {
				return fmt.Errorf("bad roles.%d.max_session_duration: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"unique_id"], aws.ToString(cloud.RoleId); got != want {
				return fmt.Errorf("bad roles.%d.unique_id: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListRoles for %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad roles.%d.create_date: got %s, want %s", i, got, want)
			}

			cloudPolicy, err := url.QueryUnescape(aws.ToString(cloud.AssumeRolePolicyDocument))
			if err != nil {
				return fmt.Errorf("parsing roles.%d.assume_role_policy: %w", i, err)
			}
			statePolicy := attrs[prefix+"assume_role_policy"]
			if !verify.PolicyStringsEquivalent(statePolicy, cloudPolicy) {
				return fmt.Errorf("bad roles.%d.assume_role_policy: got %s, want %s", i, statePolicy, cloudPolicy)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("roles ARN set size mismatch after field checks: state=%d listRoles=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccPreCheckRolesPaginationQuota confirms that the remaining RolesQuota is sufficient to create pageSize+1 roles.
func testAccPreCheckRolesPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeRolesQuota)]
	if !ok {
		t.Fatalf("RolesQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeRoles)]
	if !ok {
		t.Fatalf("Roles missing in SummaryMap: %#v", out.SummaryMap)
	}
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient RolesQuota remaining=%d need=%d (quota=%d used=%d)", remaining, need, quota, used)
	}
}

// testAccCheckRolesListDefaultPageTruncated fetches one page (without paging further) using the same empty ListRolesInput as
// the DS Read, confirming the account has more than the default MaxItems with IsTruncated=true (otherwise this case has not
// actually exercised the DS pagination path).
func testAccCheckRolesListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListRoles(ctx, &iam.ListRolesInput{})
		if err != nil {
			return fmt.Errorf("ListRoles(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListRoles page (MaxItems default %d); first page returned %d roles — DS Read would not exercise pagination", testAccIAMRolesListDefaultMaxItems, len(page.Roles))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

func testAccRolesDataSourceConfig_basicSingle(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}

data "ctyun_iam_roles" "test" {
  depends_on = [ctyun_iam_role.test]
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRolesDataSourceConfig_basicMultiple(rName string) string {
	return fmt.Sprintf(`
# Keep the ctyun_iam_role.test from step 1 to avoid a race between destroying the old role and the DS Read in the same apply
# (when depends_on only covers a/b/c, the Read may still see the lingering test role which is then deleted by Check time → state=4 listRoles=3).
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}

resource "ctyun_iam_role" "a" {
  name               = "%[1]s-a"
  assume_role_policy = %[2]s
}

resource "ctyun_iam_role" "b" {
  name               = "%[1]s-b"
  assume_role_policy = %[2]s
}

resource "ctyun_iam_role" "c" {
  name               = "%[1]s-c"
  assume_role_policy = %[2]s
}

data "ctyun_iam_roles" "test" {
  depends_on = [
    ctyun_iam_role.test,
    ctyun_iam_role.a,
    ctyun_iam_role.b,
    ctyun_iam_role.c,
  ]
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRolesDataSourceConfig_nameRegex(roleA, roleB string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "a" {
  name               = %[1]q
  assume_role_policy = %[3]s
}

resource "ctyun_iam_role" "b" {
  name               = %[2]q
  assume_role_policy = %[3]s
}

data "ctyun_iam_roles" "test" {
  name_regex = format("^%%s$", %[1]q)

  depends_on = [
    ctyun_iam_role.a,
    ctyun_iam_role.b,
  ]
}
`, roleA, roleB, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRolesDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}

data "ctyun_iam_roles" "test" {
  name_regex = "^tf-no-such-role-zzzzzzzz$"

  depends_on = [ctyun_iam_role.test]
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccRolesDataSourceConfig_pagination(rName string, roleCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  count              = %[2]d
  name               = "%[1]s-${count.index}"
  assume_role_policy = %[3]s
}

data "ctyun_iam_roles" "test" {
  name_regex = format("^%%s-", %[1]q)

  depends_on = [ctyun_iam_role.test]
}
`, rName, roleCount, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}
