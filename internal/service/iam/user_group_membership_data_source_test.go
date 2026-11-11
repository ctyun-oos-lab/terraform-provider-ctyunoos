// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
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
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMUserGroupMembershipDataSource_schema verifies the schema shape of the ctyun_iam_user_group_membership data source:
// it contains groups_for_user/id/user_name; groups_for_user is Computed; user_name has Validators.
func TestUnitIAMUserGroupMembershipDataSource_schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewUserGroupMembershipDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var resp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	for _, name := range []string{"groups_for_user", names.AttrID, names.AttrUserName} {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	groupsAttr, ok := resp.Schema.Attributes["groups_for_user"].(schema.ListAttribute)
	if !ok {
		t.Fatalf("expected groups_for_user ListAttribute, got %T", resp.Schema.Attributes["groups_for_user"])
	}
	if !groupsAttr.IsComputed() {
		t.Fatal("expected groups_for_user Computed")
	}

	userNameAttr, ok := resp.Schema.Attributes[names.AttrUserName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected user_name StringAttribute, got %T", resp.Schema.Attributes[names.AttrUserName])
	}
	if !userNameAttr.IsRequired() {
		t.Fatal("expected user_name Required")
	}
	if len(userNameAttr.Validators) == 0 {
		t.Fatal("expected user_name Validators")
	}
}

// TestUnitIAMUserGroupMembershipDataSource_nameValidation verifies the Framework Validators of the user_name input
// (covers the former TC-IAM-DUGM-04 User Validation Matrix):
// valid special characters and the upper length bound of 64 pass; empty values, 65 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMUserGroupMembershipDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewUserGroupMembershipDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}

	userNameAttr, ok := schemaResp.Schema.Attributes[names.AttrUserName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected user_name StringAttribute, got %T", schemaResp.Schema.Attributes[names.AttrUserName])
	}
	if len(userNameAttr.Validators) == 0 {
		t.Fatal("expected user_name Validators")
	}

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "valid_special_chars", value: "User_-,.+@=123", wantError: false},
		{name: "valid_special_chars_alt", value: "test-1+2_.@hello", wantError: false},
		{name: "valid_max_length", value: strings.Repeat("a", 64), wantError: false},
		{name: "empty", value: "", wantError: true},
		{name: "too_long", value: strings.Repeat("a", 65), wantError: true},
		{name: "invalid_space", value: "t est", wantError: true},
		{name: "invalid_exclamation", value: "invalid user!", wantError: true},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var diags validator.StringResponse
			for _, v := range userNameAttr.Validators {
				var resp validator.StringResponse
				v.ValidateString(ctx, validator.StringRequest{
					ConfigValue: types.StringValue(tc.value),
				}, &resp)
				diags.Diagnostics.Append(resp.Diagnostics...)
			}

			hasError := diags.Diagnostics.HasError()
			if tc.wantError && !hasError {
				t.Fatalf("expected validation error for user_name=%q, got none", tc.value)
			}
			if !tc.wantError && hasError {
				t.Fatalf("expected no validation error for user_name=%q, got: %v", tc.value, diags.Diagnostics)
			}
		})
	}
}

// TestUnitIAMUserGroupMembershipDataSource_flattenGroupsForUser verifies the flatten mapping of
// arn/create_date/group_id/group_name.
func TestUnitIAMUserGroupMembershipDataSource_flattenGroupsForUser(t *testing.T) {
	t.Parallel()

	createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
	groups := []awstypes.Group{
		{
			Arn:        aws.String("arn:ctyun:iam::969618309127:group/test1"),
			CreateDate: aws.Time(createDate),
			GroupId:    aws.String("AIDA6DQOBIAD7BL4YN5OA"),
			GroupName:  aws.String("test1"),
		},
	}

	output := tfiam.FlattenGroupsForUser(groups)

	if got, want := len(output), 1; got != want {
		t.Fatalf("expected %d groups, got %d", want, got)
	}

	if got, want := output[0].ARN.ValueString(), "arn:ctyun:iam::969618309127:group/test1"; got != want {
		t.Fatalf("expected arn %q, got %q", want, got)
	}

	if got, want := output[0].CreateDate.ValueString(), "2026-06-09T08:30:10Z"; got != want {
		t.Fatalf("expected create_date %q, got %q", want, got)
	}

	if got, want := output[0].GroupID.ValueString(), "AIDA6DQOBIAD7BL4YN5OA"; got != want {
		t.Fatalf("expected group_id %q, got %q", want, got)
	}

	if got, want := output[0].GroupName.ValueString(), "test1"; got != want {
		t.Fatalf("expected group_name %q, got %q", want, got)
	}
}

// TC-IAM-DUGM-01-Basic Membership Lookup
// verifies reading by user_name after creating a user and adding it to 1 group: the DS state matches the cloud
// ListGroupsForUser result; it also directly asserts id/user_name, plus arn/group_id of groups_for_user.0 (AttrPair with
// the fixture group) and a non-empty create_date; CheckDestroy at the end.
func TestAccIAMUserGroupMembershipDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName := rName + "-user"
	groupName := rName + "-group"
	dataSourceName := "data.ctyun_iam_user_group_membership.test"
	groupResourceName := "ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipDataSourceConfig_basic(userName, groupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, userName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, userName),
					resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.0.group_name", groupName),
					resource.TestCheckResourceAttrPair(dataSourceName, "groups_for_user.0.arn", groupResourceName, names.AttrARN),
					resource.TestCheckResourceAttrPair(dataSourceName, "groups_for_user.0.group_id", groupResourceName, "unique_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "groups_for_user.0.create_date"),
				),
			},
		},
	})
}

// TC-IAM-DUGM-02-Multiple Groups
// verifies reading after adding the same user to multiple groups: groups_for_user matches the cloud ListGroupsForUser set
// exactly; it also directly asserts id/user_name and that the list contains the two expected group_names (order-independent);
// CheckDestroy at the end.
func TestAccIAMUserGroupMembershipDataSource_multiple(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName := rName + "-user"
	groupA := rName + "-a"
	groupB := rName + "-b"
	dataSourceName := "data.ctyun_iam_user_group_membership.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipDataSourceConfig_multiple(userName, groupA, groupB),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, userName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, userName),
					resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.#", "2"),
					resource.TestCheckResourceAttrSet(dataSourceName, "groups_for_user.0.group_name"),
					resource.TestCheckResourceAttrSet(dataSourceName, "groups_for_user.1.group_name"),
					testAccCheckUserGroupMembershipDataSourceHasGroupNames(dataSourceName, groupA, groupB),
				),
			},
		},
	})
}

// TC-IAM-DUGM-03-Empty Membership
// verifies reading a user that exists but belongs to no group: groups_for_user is empty, id/user_name equal the user name,
// and the result matches the cloud ListGroupsForUser; CheckDestroy at the end.
func TestAccIAMUserGroupMembershipDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_user_group_membership.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipDataSourceConfig_empty(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, rName),
					resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DUGM-05-Missing User Negative verifies that reading a non-existent user_name returns a clear NotFound-style error,
// and that no data.ctyun_iam_user_group_membership remains in state (no half-written result).
func TestAccIAMUserGroupMembershipDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccUserGroupMembershipDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserGroupMembershipDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DUGM-05-Other Account Negative verifies that reading a user that exists under another account cannot be found by
// name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_user_group_membership left in state.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the user with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMUserGroupMembershipDataSource_otherAccount(t *testing.T) {
	crossAccountUser := testAccCrossAccountUserName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipDataSourceConfig_otherAccountUser(crossAccountUser),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccUserGroupMembershipDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserGroupMembershipDataSourceAbsentFromState(),
			},
		},
	})
}

// The ListGroupsForUser default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input + Paginator
// and cannot change MaxItems, so this case adds the user to more groups than the default page size to force the real Read through multi-page aggregation.
const testAccIAMListGroupsForUserDefaultMaxItems = 100

// TC-IAM-DUGM-06-Pagination verifies that when the number of groups a user belongs to exceeds the ListGroupsForUser default
// MaxItems, the data source Read (empty ListGroupsForUserInput → default pagination) still aggregates and returns all
// groups_for_user, matching the cloud.
func TestAccIAMUserGroupMembershipDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName := rName + "-user"
	groupNamePrefix := rName + "-group"
	groupCount := testAccIAMListGroupsForUserDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_user_group_membership.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckGroupsPaginationQuota(ctx, t, groupCount)
			testAccPreCheckGroupsPerUserQuota(ctx, t, groupCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipDataSourceConfig_pagination(userName, groupNamePrefix, groupCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserGroupMembershipListDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.#", strconv.Itoa(groupCount)),
				),
			},
		},
	})
}

// TC-IAM-DUGM-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state
// matches the cloud ListGroupsForUser result (including groups_for_user and create_date); the second apply shows no diff
// (PreApply / PostApplyPostRefresh). The configuration reuses the multiple case with memberships. CheckDestroy at the end.
func TestAccIAMUserGroupMembershipDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	userName := rName + "-user"
	groupA := rName + "-a"
	groupB := rName + "-b"
	dataSourceName := "data.ctyun_iam_user_group_membership.test"

	checks := resource.ComposeTestCheckFunc(
		testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, "groups_for_user.#", "2"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipDataSourceConfig_multiple(userName, groupA, groupB),
				Check:  checks,
			},
			{
				Config: testAccUserGroupMembershipDataSourceConfig_multiple(userName, groupA, groupB),
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

// testAccCheckUserGroupMembershipDataSourceHasGroupNames asserts that the group_name set of groups_for_user equals
// wantNames exactly (order-independent; list items are TypeList).
func testAccCheckUserGroupMembershipDataSourceHasGroupNames(dataSourceName string, wantNames ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		attrs := rs.Primary.Attributes
		count, err := strconv.Atoi(attrs["groups_for_user.#"])
		if err != nil {
			return fmt.Errorf("parsing groups_for_user.#: %w", err)
		}
		if count != len(wantNames) {
			return fmt.Errorf("groups_for_user count mismatch: got %d, want %d", count, len(wantNames))
		}

		got := make(map[string]struct{}, count)
		for i := 0; i < count; i++ {
			name := attrs[fmt.Sprintf("groups_for_user.%d.group_name", i)]
			if name == "" {
				return fmt.Errorf("empty groups_for_user.%d.group_name", i)
			}
			if _, dup := got[name]; dup {
				return fmt.Errorf("duplicate groups_for_user group_name %q", name)
			}
			got[name] = struct{}{}
		}

		for _, want := range wantNames {
			if _, ok := got[want]; !ok {
				return fmt.Errorf("expected groups_for_user to contain group_name %q, got %v", want, got)
			}
		}
		return nil
	}
}

// testAccCheckUserGroupMembershipDataSourceStateMatchesCloud checks that the DS state matches the cloud ListGroupsForUser
// result without relying on managed resource state: it compares id/user_name, plus the groups_for_user set field by field
// (arn/create_date/group_id/group_name).
func testAccCheckUserGroupMembershipDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		userName := rs.Primary.Attributes[names.AttrUserName]
		if userName == "" {
			return fmt.Errorf("bad state user_name: empty")
		}
		if rs.Primary.ID != userName {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, userName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudGroups, err := tfiam.FindGroupsForUser(ctx, conn, &iam.ListGroupsForUserInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("listing groups for user (%s): %w", userName, err)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["groups_for_user.#"])
		if err != nil {
			return fmt.Errorf("parsing groups_for_user.#: %w", err)
		}
		if stateCount != len(cloudGroups) {
			return fmt.Errorf("groups_for_user count mismatch: state=%d listGroupsForUser=%d", stateCount, len(cloudGroups))
		}

		cloudByARN := make(map[string]awstypes.Group, len(cloudGroups))
		for _, group := range cloudGroups {
			cloudByARN[aws.ToString(group.Arn)] = group
		}

		seenARN := make(map[string]struct{}, stateCount)
		for i := 0; i < stateCount; i++ {
			prefix := fmt.Sprintf("groups_for_user.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty groups_for_user.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate groups_for_user.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("groups_for_user.%d.arn %q not found in ListGroupsForUser", i, stateARN)
			}

			if got, want := attrs[prefix+"group_name"], aws.ToString(cloud.GroupName); got != want {
				return fmt.Errorf("bad groups_for_user.%d.group_name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"group_id"], aws.ToString(cloud.GroupId); got != want {
				return fmt.Errorf("bad groups_for_user.%d.group_id: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListGroupsForUser for group %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad groups_for_user.%d.create_date: got %s, want %s", i, got, want)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("groups_for_user ARN set size mismatch after field checks: state=%d listGroupsForUser=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

func testAccCheckUserGroupMembershipDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_user_group_membership.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q user_name=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrUserName],
			)
		}
		return nil
	}
}

func testAccUserGroupMembershipDataSourceConfig_basic(userName, groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  name = %[2]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.test.name
}

data "ctyun_iam_user_group_membership" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_user_group_membership.test]
}
`, userName, groupName)
}

func testAccUserGroupMembershipDataSourceConfig_multiple(userName, groupA, groupB string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_group" "a" {
  name = %[2]q
}

resource "ctyun_iam_group" "b" {
  name = %[3]q
}

resource "ctyun_iam_user_group_membership" "a" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.a.name
}

resource "ctyun_iam_user_group_membership" "b" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.b.name
}

data "ctyun_iam_user_group_membership" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [
    ctyun_iam_user_group_membership.a,
    ctyun_iam_user_group_membership.b,
  ]
}
`, userName, groupA, groupB)
}

func testAccUserGroupMembershipDataSourceConfig_empty(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

data "ctyun_iam_user_group_membership" "test" {
  user_name = ctyun_iam_user.test.name
}
`, userName)
}

func testAccUserGroupMembershipDataSourceConfig_notFound(userName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_user_group_membership" "test" {
  user_name = %[1]q
}
`, userName)
}

func testAccUserGroupMembershipDataSourceConfig_otherAccountUser(userName string) string {
	return fmt.Sprintf(`
# The cross-account user is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_user_group_membership" "test" {
  user_name = %[1]q
}
`, userName)
}

func testAccUserGroupMembershipDataSourceConfig_pagination(userName, groupNamePrefix string, groupCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  count = %[3]d
  name  = "%[2]s-${count.index}"
}

resource "ctyun_iam_user_group_membership" "test" {
  count = %[3]d
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.test[count.index].name
}

data "ctyun_iam_user_group_membership" "test" {
  user_name = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_user_group_membership.test]
}
`, userName, groupNamePrefix, groupCount)
}

func testAccUserGroupMembershipDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed user_group_membership data source read left no residual state
`
}

// testAccPreCheckGroupsPerUserQuota confirms that GroupsPerUserQuota is sufficient for a single user to join need groups.
func testAccPreCheckGroupsPerUserQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroupsPerUserQuota)]
	if !ok {
		t.Fatalf("GroupsPerUserQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	if int(quota) < need {
		t.Skipf("insufficient GroupsPerUserQuota=%d need=%d", quota, need)
	}
}

// testAccCheckUserGroupMembershipListDefaultPageTruncated fetches one page (without paging further) using the same empty
// ListGroupsForUserInput as the DS Read, confirming the user belongs to more groups than the default MaxItems with
// IsTruncated=true (otherwise this case has not actually exercised the DS pagination path).
func testAccCheckUserGroupMembershipListDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		userName := rs.Primary.Attributes[names.AttrUserName]
		if userName == "" {
			return fmt.Errorf("bad user_name: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("ListGroupsForUser(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListGroupsForUser page (MaxItems default %d); first page returned %d groups — DS Read would not exercise pagination", testAccIAMListGroupsForUserDefaultMaxItems, len(page.Groups))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}
