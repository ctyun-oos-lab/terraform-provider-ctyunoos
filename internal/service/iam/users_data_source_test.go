// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
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
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMUsersDataSource_schema verifies the schema shape of ctyun_iam_users:
// it contains arns/names/name_regex/users; it does not contain the upstream single-resource user_name/access_key_id;
// the users elements contain arn/create_date/password_last_used/user_id/user_name, but not access_key_count etc.;
// name_regex has a ValidateFunc.
func TestUnitIAMUsersDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceUsers()

	for _, name := range []string{names.AttrARNs, names.AttrNames, "name_regex", "users"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	for _, name := range []string{"access_key_id", names.AttrUserName} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("unexpected %s schema", name)
		}
	}

	userSchema := dataSource.Schema["users"].Elem.(*schema.Resource).Schema
	for _, name := range []string{
		names.AttrARN,
		"create_date",
		"password_last_used",
		"user_id",
		names.AttrUserName,
	} {
		if _, ok := userSchema[name]; !ok {
			t.Fatalf("expected users.%s schema", name)
		}
	}
	for _, name := range []string{"access_key_count", "mfa_device_count", "password_create_date"} {
		if _, ok := userSchema[name]; ok {
			t.Fatalf("unexpected users.%s schema", name)
		}
	}

	if dataSource.Schema["name_regex"].ValidateFunc == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}
}

// TestUnitIAMUsersDataSource_nameRegexValidation verifies the schema ValidateFunc of name_regex:
// valid regular expressions pass; various invalid regex forms return clear parsing errors (covers the former TC-IAM-DUSERS-04).
func TestUnitIAMUsersDataSource_nameRegexValidation(t *testing.T) {
	t.Parallel()

	validate := tfiam.DataSourceUsers().Schema["name_regex"].ValidateFunc
	if validate == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_prefix", value: "^tf-", errContains: nil},
		{name: "valid_exact", value: `^my-user-123$`, errContains: nil},
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

// TestUnitIAMUsersDataSource_flattenUsers verifies flattenUsers:
// the normal path maps arn/create_date/password_last_used/user_id/user_name plus the arns/names slices;
// multiple users keep the input order; empty input returns a non-nil empty users slice with arns/names nil.
func TestUnitIAMUsersDataSource_flattenUsers(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		users := []awstypes.User{
			{
				Arn:              aws.String("arn:ctyun:iam::969618309127:user/1111"),
				CreateDate:       aws.Time(createDate),
				PasswordLastUsed: aws.Time(createDate),
				UserId:           aws.String("AIDA6DQOBIAD7BL4YN5OA"),
				UserName:         aws.String("1111"),
			},
		}

		tfUsers, arns, userNames := tfiam.FlattenUsers(users)

		if got, want := len(tfUsers), 1; got != want {
			t.Fatalf("expected %d user, got %d", want, got)
		}
		if got, want := arns[0], "arn:ctyun:iam::969618309127:user/1111"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}
		if got, want := userNames[0], "1111"; got != want {
			t.Fatalf("expected user name %q, got %q", want, got)
		}
		if got, want := tfUsers[0]["user_id"], "AIDA6DQOBIAD7BL4YN5OA"; got != want {
			t.Fatalf("expected user_id %q, got %q", want, got)
		}
		if got, want := tfUsers[0]["create_date"], "2026-06-09T08:30:10Z"; got != want {
			t.Fatalf("expected create_date %q, got %q", want, got)
		}
		if got, want := tfUsers[0]["password_last_used"], "2026-06-09T08:30:10Z"; got != want {
			t.Fatalf("expected password_last_used %q, got %q", want, got)
		}
		if got, want := tfUsers[0][names.AttrUserName], "1111"; got != want {
			t.Fatalf("expected user_name %q, got %q", want, got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		users := []awstypes.User{
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:user/1111"),
				CreateDate: aws.Time(createDate),
				UserId:     aws.String("AIDA1111"),
				UserName:   aws.String("1111"),
			},
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:user/Terraform-1"),
				CreateDate: aws.Time(createDate),
				UserId:     aws.String("AIDATERRAFORM1"),
				UserName:   aws.String("Terraform-1"),
			},
		}

		tfUsers, arns, userNames := tfiam.FlattenUsers(users)
		if got, want := len(tfUsers), 2; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		if got, want := len(arns), 2; got != want {
			t.Fatalf("expected %d arns, got %d", want, got)
		}
		if got, want := len(userNames), 2; got != want {
			t.Fatalf("expected %d names, got %d", want, got)
		}
		for i, user := range users {
			if got, want := arns[i], aws.ToString(user.Arn); got != want {
				t.Fatalf("expected arns[%d] %q, got %q", i, want, got)
			}
			if got, want := userNames[i], aws.ToString(user.UserName); got != want {
				t.Fatalf("expected names[%d] %q, got %q", i, want, got)
			}
			if got, want := tfUsers[i][names.AttrUserName], aws.ToString(user.UserName); got != want {
				t.Fatalf("expected users[%d].user_name %q, got %q", i, want, got)
			}
			if got, want := tfUsers[i][names.AttrARN], aws.ToString(user.Arn); got != want {
				t.Fatalf("expected users[%d].arn %q, got %q", i, want, got)
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		tfUsers, arns, userNames := tfiam.FlattenUsers(nil)
		if tfUsers == nil {
			t.Fatal("expected non-nil empty users slice for nil input")
		}
		if got, want := len(tfUsers), 0; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		if arns != nil {
			t.Fatalf("expected arns nil, got %#v", arns)
		}
		if userNames != nil {
			t.Fatalf("expected names nil, got %#v", userNames)
		}

		tfUsers, arns, userNames = tfiam.FlattenUsers([]awstypes.User{})
		if tfUsers == nil {
			t.Fatal("expected non-nil empty users slice for empty input")
		}
		if got, want := len(tfUsers), 0; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		if arns != nil {
			t.Fatalf("expected arns nil, got %#v", arns)
		}
		if userNames != nil {
			t.Fatalf("expected names nil, got %#v", userNames)
		}
	})
}

// TestUnitIAMUsersDataSource_filterUsersByNameRegex verifies local name_regex filtering:
// partial match, full match (output = input), no match at all (nil empty slice), and empty input.
func TestUnitIAMUsersDataSource_filterUsersByNameRegex(t *testing.T) {
	t.Parallel()

	users := []awstypes.User{
		{UserName: aws.String("1111")},
		{UserName: aws.String("Terraform-1")},
	}

	t.Run("partial_match", func(t *testing.T) {
		t.Parallel()

		filtered := tfiam.FilterUsersByNameRegex(users, regexp.MustCompile(`^Terraform`))
		if got, want := len(filtered), 1; got != want {
			t.Fatalf("expected %d user, got %d", want, got)
		}
		if got, want := aws.ToString(filtered[0].UserName), "Terraform-1"; got != want {
			t.Fatalf("expected user_name %q, got %q", want, got)
		}
	})

	t.Run("all_match", func(t *testing.T) {
		t.Parallel()

		filtered := tfiam.FilterUsersByNameRegex(users, regexp.MustCompile(`.*`))
		if got, want := len(filtered), len(users); got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		for i := range users {
			if got, want := aws.ToString(filtered[i].UserName), aws.ToString(users[i].UserName); got != want {
				t.Fatalf("expected user_name[%d] %q, got %q", i, want, got)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		t.Parallel()

		filtered := tfiam.FilterUsersByNameRegex(users, regexp.MustCompile(`^nomatch-`))
		if filtered != nil {
			t.Fatalf("expected nil slice for no match, got %#v", filtered)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()

		filtered := tfiam.FilterUsersByNameRegex(nil, regexp.MustCompile(`.*`))
		if filtered != nil {
			t.Fatalf("expected nil slice for nil input, got %#v", filtered)
		}
		filtered = tfiam.FilterUsersByNameRegex([]awstypes.User{}, regexp.MustCompile(`.*`))
		if filtered != nil {
			t.Fatalf("expected nil slice for empty input, got %#v", filtered)
		}
	})
}

// TC-IAM-DUSERS-01-Basic List (incorporating the former 03 User Field Readback field checks):
// Sub-scenario 1: create only 1 user without name_regex; id is the Region, and arns/names/users match the full ListUsers
// result exactly (no more, no less), with each user's fields matching ListUsers.
// Sub-scenario 2: create multiple users; same checks as sub-scenario 1. CheckDestroy at the end.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account; if run in parallel with
// pagination etc., the snapshot read at Apply would differ from the List count at Check time.
func TestAccIAMUsersDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_users.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig_basicSingle(rName),
				Check:  testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, ""),
			},
			{
				Config: testAccUsersDataSourceConfig_basicMultiple(rName),
				Check:  testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, ""),
			},
		},
	})
}

// TC-IAM-DUSERS-02-NameRegex Filter verifies creating two users and using name_regex to match exactly one of them:
// ExactMatch compares against "ListUsers + local filtering with the same rule"; it also explicitly asserts that the
// users/names/arns counts are all 1 and that names contains the expected userA; CheckDestroy at the end.
func TestAccIAMUsersDataSource_nameRegex(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userA := rName + "-a"
	userB := rName + "-b"
	dataSourceName := "data.ctyun_iam_users.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(userA))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig_nameRegex(userA, userB),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", userA),
				),
			},
		},
	})
}

// TC-IAM-DUSERS-05-Empty Result verifies: first create at least 1 user (ensuring ListUsers is non-empty),
// then read with a name_regex that cannot match anything; ExactMatch compares against the filtered ListUsers result and
// explicitly asserts that users/arns/names are all empty (due to regex filtering, not because the cloud has no users),
// with id still being the Region; CheckDestroy at the end.
func TestAccIAMUsersDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_users.test"
	nameRegex := "^tf-no-such-user-zzzzzzzz$"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "0"),
				),
			},
		},
	})
}

// The ListUsers default MaxItems is 100, consistent with AWS IAM; the DS Read uses an empty Input + Paginator and cannot
// change MaxItems, so this case creates more data than the default page size to force the real Read through multi-page aggregation.
const testAccIAMUsersListDefaultMaxItems = 100

// TC-IAM-DUSERS-06-Pagination verifies that when the total number of users in the account exceeds the ListUsers default
// MaxItems, the data source Read (empty ListUsersInput → default pagination) still returns all users matching name_regex.
func TestAccIAMUsersDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	userCount := testAccIAMUsersListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_users.test"
	nameRegex := fmt.Sprintf("^%s-", regexp.QuoteMeta(rName))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckUsersPaginationQuota(ctx, t, userCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig_pagination(rName, userCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUsersListDefaultPageTruncated(ctx, t),
					testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", strconv.Itoa(userCount)),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", strconv.Itoa(userCount)),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", strconv.Itoa(userCount)),
				),
			},
		},
	})
}

// TC-IAM-DUSERS-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state is
// ExactMatch-compared against the cloud ListUsers (filtered with the same name_regex); the second apply shows no diff
// (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused by unstable read-back formats of
// flattenUsers / create_date / password_last_used.
// name_regex is used to bound the result set so the test can run in parallel.
func TestAccIAMUsersDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userA := rName + "-a"
	userB := rName + "-b"
	dataSourceName := "data.ctyun_iam_users.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(userA))

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckUsersDataSourceExactMatchListUsers(ctx, t, dataSourceName, nameRegex),
		resource.TestCheckResourceAttr(dataSourceName, "users.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
		resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", userA),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig_nameRegex(userA, userB),
				Check:  checks,
			},
			{
				Config: testAccUsersDataSourceConfig_nameRegex(userA, userB),
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

// testAccCheckUsersDataSourceExactMatchListUsers checks the DS state:
//   - id equals the current IAM EffectiveRealRegion;
//   - the cloud list is fetched with ListUsers (same source as the DS); if nameRegex is non-empty, it is filtered locally with
//     the same rule as the DS;
//   - arns/names/users match the filtered result set exactly (no more, no less), with field-by-field comparison.
func testAccCheckUsersDataSourceExactMatchListUsers(ctx context.Context, t *testing.T, dataSourceName, nameRegex string) resource.TestCheckFunc {
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
		stateCount, err := strconv.Atoi(attrs["users.#"])
		if err != nil {
			return fmt.Errorf("parsing users.#: %w", err)
		}

		conn := meta.IAMClient(ctx)
		cloudUsers, _, err := tfiam.FindUsers(ctx, conn, &iam.ListUsersInput{})
		if err != nil {
			return fmt.Errorf("listing IAM Users: %w", err)
		}
		if nameRegex != "" {
			r, err := regexp.Compile(nameRegex)
			if err != nil {
				return fmt.Errorf("compiling name_regex (%s): %w", nameRegex, err)
			}
			cloudUsers = tfiam.FilterUsersByNameRegex(cloudUsers, r)
		}

		if stateCount != len(cloudUsers) {
			return fmt.Errorf("users count mismatch: state=%d listUsers=%d", stateCount, len(cloudUsers))
		}

		cloudByARN := make(map[string]awstypes.User, len(cloudUsers))
		cloudARNs := make(map[string]struct{}, len(cloudUsers))
		cloudNames := make(map[string]struct{}, len(cloudUsers))
		for _, user := range cloudUsers {
			arn := aws.ToString(user.Arn)
			cloudByARN[arn] = user
			cloudARNs[arn] = struct{}{}
			cloudNames[aws.ToString(user.UserName)] = struct{}{}
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
			prefix := fmt.Sprintf("users.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty users.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate users.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("users.%d.arn %q not found in ListUsers", i, stateARN)
			}

			if got, want := attrs[prefix+names.AttrUserName], aws.ToString(cloud.UserName); got != want {
				return fmt.Errorf("bad users.%d.user_name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"user_id"], aws.ToString(cloud.UserId); got != want {
				return fmt.Errorf("bad users.%d.user_id: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListUsers for %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad users.%d.create_date: got %s, want %s", i, got, want)
			}

			wantPasswordLastUsed := ""
			if cloud.PasswordLastUsed != nil {
				wantPasswordLastUsed = aws.ToTime(cloud.PasswordLastUsed).Format(time.RFC3339)
			}
			if got := attrs[prefix+"password_last_used"]; got != wantPasswordLastUsed {
				return fmt.Errorf("bad users.%d.password_last_used: got %s, want %s", i, got, wantPasswordLastUsed)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("users ARN set size mismatch after field checks: state=%d listUsers=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccPreCheckUsersPaginationQuota confirms that the remaining UsersQuota is sufficient to create pageSize+1 users.
func testAccPreCheckUsersPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeUsersQuota)]
	if !ok {
		t.Fatalf("UsersQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeUsers)]
	if !ok {
		t.Fatalf("Users missing in SummaryMap: %#v", out.SummaryMap)
	}
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient UsersQuota remaining=%d need=%d (quota=%d used=%d)", remaining, need, quota, used)
	}
}

// testAccCheckUsersListDefaultPageTruncated fetches one page (without paging further) using the same empty ListUsersInput as
// the DS Read, confirming the account has more than the default MaxItems with IsTruncated=true (otherwise this case has not
// actually exercised the DS pagination path).
func testAccCheckUsersListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListUsers(ctx, &iam.ListUsersInput{})
		if err != nil {
			return fmt.Errorf("ListUsers(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListUsers page (MaxItems default %d); first page returned %d users — DS Read would not exercise pagination", testAccIAMUsersListDefaultMaxItems, len(page.Users))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

func testAccUsersDataSourceConfig_basicSingle(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

data "ctyun_iam_users" "test" {
  depends_on = [ctyun_iam_user.test]
}
`, rName)
}

func testAccUsersDataSourceConfig_basicMultiple(rName string) string {
	return fmt.Sprintf(`
# Keep the ctyun_iam_user.test from step 1 to avoid a race between destroying the old user and the DS Read in the same apply.
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_user" "a" {
  name          = "%[1]s-a"
  force_destroy = true
}

resource "ctyun_iam_user" "b" {
  name          = "%[1]s-b"
  force_destroy = true
}

resource "ctyun_iam_user" "c" {
  name          = "%[1]s-c"
  force_destroy = true
}

data "ctyun_iam_users" "test" {
  depends_on = [
    ctyun_iam_user.test,
    ctyun_iam_user.a,
    ctyun_iam_user.b,
    ctyun_iam_user.c,
  ]
}
`, rName)
}

func testAccUsersDataSourceConfig_nameRegex(userA, userB string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "a" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_user" "b" {
  name          = %[2]q
  force_destroy = true
}

data "ctyun_iam_users" "test" {
  name_regex = format("^%%s$", %[1]q)

  depends_on = [
    ctyun_iam_user.a,
    ctyun_iam_user.b,
  ]
}
`, userA, userB)
}

func testAccUsersDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

data "ctyun_iam_users" "test" {
  name_regex = "^tf-no-such-user-zzzzzzzz$"

  depends_on = [ctyun_iam_user.test]
}
`, rName)
}

func testAccUsersDataSourceConfig_pagination(rName string, userCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  count         = %[2]d
  name          = "%[1]s-${count.index}"
  force_destroy = true
}

data "ctyun_iam_users" "test" {
  name_regex = format("^%%s-", %[1]q)

  depends_on = [ctyun_iam_user.test]
}
`, rName, userCount)
}
