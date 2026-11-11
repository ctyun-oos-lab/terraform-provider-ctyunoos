// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMGroupDataSource_schema verifies the schema shape of the ctyun_iam_group data source:
// it contains arn/group_id/group_name/users; group_name is Required and has a ValidateFunc;
// users is a Computed TypeList whose elements contain arn/create_date/password_last_used/user_id/user_name.
func TestUnitIAMGroupDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceGroup()

	for _, name := range []string{
		names.AttrARN,
		"group_id",
		names.AttrGroupName,
		"users",
	} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	if !dataSource.Schema[names.AttrGroupName].Required {
		t.Fatal("expected group_name to be Required")
	}
	if dataSource.Schema[names.AttrGroupName].ValidateFunc == nil {
		t.Fatal("expected group_name ValidateFunc")
	}

	users := dataSource.Schema["users"]
	if users.Type != schema.TypeList {
		t.Fatalf("expected users TypeList, got %v", users.Type)
	}
	if !users.Computed {
		t.Fatal("expected users Computed")
	}

	userSchema := users.Elem.(*schema.Resource).Schema
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
}

// TestUnitIAMGroupDataSource_nameValidation verifies the ValidateFunc of the ctyun_iam_group data source
// input group_name (covers the former TC-IAM-DGROUP-04 Name Validation Matrix):
// valid special characters and the upper length bound of 128 pass; empty values, 129 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMGroupDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	validateFunc := tfiam.DataSourceGroup().Schema[names.AttrGroupName].ValidateFunc
	if validateFunc == nil {
		t.Fatal("expected group_name ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_special_chars", value: "Group_-,.+@=123", errContains: nil},
		{name: "valid_max_length", value: strings.Repeat("a", 128), errContains: nil},
		{name: "empty", value: "", errContains: []string{
			"expected length of group_name to be in the range (1 - 128)",
		}},
		{name: "too_long", value: strings.Repeat("a", 129), errContains: []string{
			"expected length of group_name to be in the range (1 - 128)",
		}},
		{name: "invalid_space", value: "t est", errContains: []string{
			"invalid value for group_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "invalid_exclamation", value: "group!", errContains: []string{
			"invalid value for group_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", errContains: []string{
			"invalid value for group_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validateFunc(tc.value, names.AttrGroupName)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for group_name=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for group_name=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for group_name missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TC-IAM-DGROUP-01-Basic Lookup verifies reading by group_name after creating a group (with no members):
// the DS state matches the cloud GetGroup result (id/group_id/group_name/arn/users; not compared against
// managed resource state); it also explicitly asserts that the users list is empty; CheckDestroy at the end.
func TestAccIAMGroupDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DGROUP-02-Group With Users verifies reading when the group has members:
// the users list fields (arn/create_date/password_last_used/user_id/user_name) are compared against the cloud GetGroup result; CheckDestroy at the end.
func TestAccIAMGroupDataSource_users(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userNamePrefix := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userCount := 1
	dataSourceName := "data.ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupDataSourceConfig_user(groupName, userNamePrefix, userCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", strconv.Itoa(userCount)),
				),
			},
		},
	})
}

// The GetGroup default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input + Paginator
// and cannot change MaxItems, so this case creates more members than the default page size to force the real Read through multi-page aggregation.
const testAccIAMGroupGetDefaultMaxItems = 100

// TC-IAM-DGROUP-03-Pagination verifies that when the number of group members exceeds the GetGroup default MaxItems,
// the data source Read (empty GetGroupInput → default pagination) still aggregates and returns all users, matching the cloud.
func TestAccIAMGroupDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userNamePrefix := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userCount := testAccIAMGroupGetDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckUsersPaginationQuota(ctx, t, userCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupDataSourceConfig_user(groupName, userNamePrefix, userCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupGetDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", strconv.Itoa(userCount)),
				),
			},
		},
	})
}

// TC-IAM-DGROUP-05-NotFound Negative verifies that reading a non-existent group_name returns a clear NotFound-style error,
// and that no data.ctyun_iam_group remains in state (no half-written result).
func TestAccIAMGroupDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccGroupDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckGroupDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DGROUP-05-Other Account Negative verifies that reading a group that exists under another account cannot be found
// by name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_group left in state.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the group with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMGroupDataSource_otherAccount(t *testing.T) {
	crossAccountGroup := testAccCrossAccountGroupName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupDataSourceConfig_otherAccountGroup(crossAccountGroup),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccGroupDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckGroupDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DGROUP-06-Membership Drift Refresh verifies that after externally changing group membership, the DS reads back
// the latest list via Refresh:
// 1) create an empty-member group+user+DS, assert users.#=0 against the cloud, then externally AddUserToGroup;
// 2) RefreshState → matches the cloud with users.#=1 containing that user;
// 3) externally RemoveUserFromGroup;
// 4) RefreshState → matches the cloud with users.#=0.
func TestAccIAMGroupDataSource_membershipDriftRefresh(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_group.test"
	groupResourceName := "ctyun_iam_group.group"
	userResourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckGroupDataSourceCleanupExternalMembership(ctx, t, groupResourceName, userResourceName),
			testAccCheckGroupDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupDataSourceConfig_driftRefresh(groupName, userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "0"),
					testAccCheckGroupDataSourceAddUserExternal(ctx, t, groupResourceName, userResourceName),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "users.0.user_name", userName),
				),
			},
			{
				Config: testAccGroupDataSourceConfig_driftRefresh(groupName, userName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceRemoveUserExternal(ctx, t, groupResourceName, userResourceName),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DGROUP-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud GetGroup result (including the users list and create_date / password_last_used); the second apply
// shows no diff (PreApply / PostApplyPostRefresh). The configuration reuses the case with members in the group. CheckDestroy at the end.
func TestAccIAMGroupDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userNamePrefix := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userCount := 1
	dataSourceName := "data.ctyun_iam_group.test"

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckGroupDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, "users.#", strconv.Itoa(userCount)),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupDataSourceConfig_user(groupName, userNamePrefix, userCount),
				Check:  checks,
			},
			{
				Config: testAccGroupDataSourceConfig_user(groupName, userNamePrefix, userCount),
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

// testAccCheckGroupDataSourceStateMatchesCloud checks that the data source state matches the cloud GetGroup result
// (paginated aggregation, same-source middleware), without relying on managed resource state: it compares
// id/group_id/group_name/arn, plus the users set field by field (arn/user_name/user_id/create_date/password_last_used).
func testAccCheckGroupDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		groupName := rs.Primary.Attributes[names.AttrGroupName]
		if groupName == "" {
			return fmt.Errorf("bad state group_name: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudGroup, cloudUsers, err := testAccGetGroupWithUsers(ctx, conn, groupName)
		if err != nil {
			return err
		}

		wantGroupID := aws.ToString(cloudGroup.GroupId)
		wantGroupName := aws.ToString(cloudGroup.GroupName)

		if rs.Primary.ID != wantGroupID {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, wantGroupID)
		}
		if got := rs.Primary.Attributes["group_id"]; got != wantGroupID {
			return fmt.Errorf("bad group_id: got %s, want %s", got, wantGroupID)
		}
		if got := rs.Primary.Attributes[names.AttrGroupName]; got != wantGroupName {
			return fmt.Errorf("bad group_name: got %s, want %s", got, wantGroupName)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], aws.ToString(cloudGroup.Arn); got != want {
			return fmt.Errorf("bad arn: got %s, want %s", got, want)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["users.#"])
		if err != nil {
			return fmt.Errorf("parsing users.#: %w", err)
		}
		if stateCount != len(cloudUsers) {
			return fmt.Errorf("users count mismatch: state=%d getGroup=%d", stateCount, len(cloudUsers))
		}

		cloudByARN := make(map[string]awstypes.User, len(cloudUsers))
		for _, user := range cloudUsers {
			cloudByARN[aws.ToString(user.Arn)] = user
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
				return fmt.Errorf("users.%d.arn %q not found in GetGroup", i, stateARN)
			}

			if got, want := attrs[prefix+names.AttrUserName], aws.ToString(cloud.UserName); got != want {
				return fmt.Errorf("bad users.%d.user_name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"user_id"], aws.ToString(cloud.UserId); got != want {
				return fmt.Errorf("bad users.%d.user_id: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in GetGroup for user %s", stateARN)
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
			return fmt.Errorf("users ARN set size mismatch after field checks: state=%d getGroup=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccCheckGroupGetDefaultPageTruncated fetches one page (without paging further) using the same empty GetGroupInput
// as the DS Read, confirming the group has more members than the default MaxItems with IsTruncated=true (otherwise this
// case has not actually exercised the DS pagination path).
func testAccCheckGroupGetDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		groupName := rs.Primary.Attributes[names.AttrGroupName]
		if groupName == "" {
			return fmt.Errorf("bad group_name: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.GetGroup(ctx, &iam.GetGroupInput{
			GroupName: aws.String(groupName),
		}, tfiam.WithOOSIAMGetGroupResponse())
		if err != nil {
			return fmt.Errorf("GetGroup(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default GetGroup page (MaxItems default %d); first page returned %d users — DS Read would not exercise pagination", testAccIAMGroupGetDefaultMaxItems, len(page.Users))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

// testAccGetGroupWithUsers shares the same source as the DS Read: GetGroupPaginator + OOS GetGroup middleware, aggregating all Users.
func testAccGetGroupWithUsers(ctx context.Context, conn *iam.Client, groupName string) (*awstypes.Group, []awstypes.User, error) {
	input := &iam.GetGroupInput{
		GroupName: aws.String(groupName),
	}

	var group *awstypes.Group
	var users []awstypes.User
	pages := iam.NewGetGroupPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, tfiam.WithOOSIAMGetGroupResponse())
		if err != nil {
			return nil, nil, fmt.Errorf("reading IAM Group (%s): %w", groupName, err)
		}
		if group == nil {
			group = page.Group
		}
		users = append(users, page.Users...)
	}

	if group == nil {
		return nil, nil, fmt.Errorf("IAM Group (%s) not found in cloud", groupName)
	}

	return group, users, nil
}

func testAccGroupDataSourceConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "group" {
  name = %[1]q
}

data "ctyun_iam_group" "test" {
  group_name = ctyun_iam_group.group.name
}
`, name)
}

func testAccGroupDataSourceConfig_driftRefresh(groupName, userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "group" {
  name = %[1]q
}

resource "ctyun_iam_user" "user" {
  name          = %[2]q
  force_destroy = true
}

data "ctyun_iam_group" "test" {
  group_name = ctyun_iam_group.group.name
}
`, groupName, userName)
}

// testAccCheckGroupDataSourceAddUserExternal uses the API during the Check phase to add the TF-managed user to the group (bypassing the membership resource).
func testAccCheckGroupDataSourceAddUserExternal(ctx context.Context, t *testing.T, groupResourceName, userResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", groupResourceName)
		}
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", userResourceName)
		}

		groupName := groupRS.Primary.Attributes[names.AttrName]
		userName := userRS.Primary.Attributes[names.AttrName]
		if groupName == "" || userName == "" {
			return fmt.Errorf("empty group/user name for external AddUserToGroup")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.AddUserToGroup(ctx, &iam.AddUserToGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("adding IAM User (%s) to Group (%s): %w", userName, groupName, err)
		}

		_, err = tfiam.FindGroupForUserByTwoPartKey(ctx, conn, userName, groupName)
		if err != nil {
			return fmt.Errorf("reading IAM User Group Membership (%s/%s): %w", userName, groupName, err)
		}

		return nil
	}
}

// testAccCheckGroupDataSourceRemoveUserExternal uses the API during the Check phase to remove the user from the group.
func testAccCheckGroupDataSourceRemoveUserExternal(ctx context.Context, t *testing.T, groupResourceName, userResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", groupResourceName)
		}
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", userResourceName)
		}

		groupName := groupRS.Primary.Attributes[names.AttrName]
		userName := userRS.Primary.Attributes[names.AttrName]
		if groupName == "" || userName == "" {
			return fmt.Errorf("empty group/user name for external RemoveUserFromGroup")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("removing IAM User (%s) from Group (%s): %w", userName, groupName, err)
		}

		return nil
	}
}

// testAccCheckGroupDataSourceCleanupExternalMembership makes a best effort to detach externally added group membership before destroy, so leftover bindings do not block DeleteUser/DeleteGroup.
func testAccCheckGroupDataSourceCleanupExternalMembership(ctx context.Context, t *testing.T, groupResourceName, userResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return nil
		}
		userRS, ok := s.RootModule().Resources[userResourceName]
		if !ok {
			return nil
		}

		groupName := groupRS.Primary.Attributes[names.AttrName]
		userName := userRS.Primary.Attributes[names.AttrName]
		if groupName == "" || userName == "" {
			return nil
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(userName),
		})
		if err != nil &&
			!errs.IsA[*awstypes.NoSuchEntityException](err) &&
			!tfawserr.ErrHTTPStatusCodeEquals(err, 404) {
			// Errors other than "no longer in the group": log only and leave them to the subsequent Destroy check
			t.Logf("cleanup RemoveUserFromGroup(%s/%s): %s", userName, groupName, err)
		}

		return nil
	}
}

func testAccGroupDataSourceConfig_user(groupName, userName string, userCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "group" {
  name = %[1]q
}

resource "ctyun_iam_user" "user" {
  name          = "%[2]s-${count.index}"
  count         = %[3]d
  force_destroy = true
}

resource "ctyun_iam_user_group_membership" "team" {
  count = %[3]d
  user  = ctyun_iam_user.user[count.index].name
  group = ctyun_iam_group.group.name
}

data "ctyun_iam_group" "test" {
  group_name = ctyun_iam_group.group.name

  depends_on = [ctyun_iam_user_group_membership.team]
}
`, groupName, userName, userCount)
}

func testAccGroupDataSourceConfig_notFound(groupName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_group" "test" {
  group_name = %[1]q
}
`, groupName)
}

func testAccGroupDataSourceConfig_otherAccountGroup(groupName string) string {
	return fmt.Sprintf(`
# The cross-account group is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_group" "test" {
  group_name = %[1]q
}
`, groupName)
}

func testAccGroupDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed group data source read left no residual state
`
}

func testAccCheckGroupDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_group.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q group_name=%q arn=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrGroupName],
				rs.Primary.Attributes[names.AttrARN],
			)
		}
		return nil
	}
}
