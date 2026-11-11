// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const groupTestResourcePrefix = acctest.ResourcePrefix + "-group"

// TC-IAM-GROUP-01-Basic CRUD verifies basic group creation, remote read, state population, import, and destruction.
func TestAccIAMGroup_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.Group
	rName := acctest.RandomWithPrefix(t, groupTestResourcePrefix)
	resourceName := "ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupExists(ctx, t, resourceName, &conf),
					testAccCheckGroupState(resourceName, &conf, rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies local length and character-set validation for group names.
func TestUnitIAMGroup_ResourceGroupNameValidation(t *testing.T) {
	t.Parallel()

	nameSchema, ok := tfiam.ResourceGroup().Schema[names.AttrName]
	if !ok || nameSchema == nil {
		t.Fatal("expected name schema")
	}
	if !nameSchema.Required {
		t.Fatal("expected name to be required")
	}
	if nameSchema.Optional {
		t.Fatal("expected name not to be optional")
	}
	if nameSchema.Computed {
		t.Fatal("expected name not to be computed")
	}
	if !nameSchema.ForceNew {
		t.Fatal("expected name changes to force a new group")
	}

	validate := nameSchema.ValidateFunc
	if validate == nil {
		t.Fatal("expected name validation")
	}

	testCases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid_uppercase", value: "GROUP", wantErr: false},
		{name: "valid_lowercase", value: "group", wantErr: false},
		{name: "valid_numeric", value: "123456", wantErr: false},
		{name: "valid_special_characters", value: "Group=,.@-_+", wantErr: false},
		{name: "valid_min_length", value: "a", wantErr: false},
		{name: "valid_max_length", value: strings.Repeat("a", 128), wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 129), wantErr: true},
		{name: "contains_space", value: "invalid group", wantErr: true},
		{name: "contains_exclamation", value: "invalid group!", wantErr: true},
		{name: "contains_slash", value: "invalid/group", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validate(tc.value, names.AttrName)
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; errors: %v", gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies that an empty group name is rejected by the provider schema in ACC.
func TestAccIAMGroup_nameLengthValidation_empty(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfig_basic(""),
				ExpectError: regexache.MustCompile(`expected length of name to be in the range \(1 - 128\), got`),
			},
		},
	})
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies that a 129-character group name is rejected by the provider schema in ACC.
func TestAccIAMGroup_nameLengthValidation_tooLong(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfig_basic(strings.Repeat("a", 129)),
				ExpectError: regexache.MustCompile(`expected length of name to be in the range \(1 - 128\), got`),
			},
		},
	})
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies that a group name containing spaces is rejected by the provider schema in ACC.
func TestAccIAMGroup_nameInvalidCharacters_space(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfig_basic("invalid group"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies that a group name containing a slash is rejected by the provider schema in ACC.
func TestAccIAMGroup_nameInvalidCharacters_invalidCharacter(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfig_basic("invalid/group"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-GROUP-02-Name Validation Matrix verifies that a duplicate name returns an explicit conflict error during service-side creation.
func TestAccIAMGroup_duplicateName(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, groupTestResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfig_duplicateName(rName),
				ExpectError: regexache.MustCompile(`(?i)(already exists|EntityAlreadyExists|Duplicate)`),
			},
		},
	})
}

// TC-IAM-GROUP-03-Rename verifies that changing name triggers ForceNew replacement and the new object matches remote state.
func TestAccIAMGroup_nameChange(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.Group
	rName1 := acctest.RandomWithPrefix(t, groupTestResourcePrefix)
	rName2 := acctest.RandomWithPrefix(t, groupTestResourcePrefix)
	resourceName := "ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig_basic(rName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, t, resourceName, &conf),
					testAccCheckGroupState(resourceName, &conf, rName1),
				),
			},
			{
				Config: testAccGroupConfig_basic(rName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, t, resourceName, &conf),
					testAccCheckGroupState(resourceName, &conf, rName2),
					testAccCheckGroupDeleted(ctx, t, rName1),
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

// TC-IAM-GROUP-04-Disappears verifies that after remote group deletion, Terraform detects the disappearance and produces a non-empty plan.
func TestAccIAMGroup_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.Group
	rName := acctest.RandomWithPrefix(t, groupTestResourcePrefix)
	resourceName := "ctyun_iam_group.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, t, resourceName, &conf),
					testAccCheckGroupState(resourceName, &conf, rName),
					testAccCheckGroupDisappearsExternally(ctx, t, resourceName),
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
				Check:              testAccCheckGroupRemovedFromState(resourceName),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// TC-IAM-GROUP-05-Delete With Members Negative verifies that deletion fails with an explicit error while external memberships remain.
func TestAccIAMGroup_deleteWithMembers(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.Group
	groupName := acctest.RandomWithPrefix(t, groupTestResourcePrefix)
	resourceName := "ctyun_iam_group.test"

	acctest.PreCheck(ctx, t)
	fixture := testAccCreateGroupMemberFixture(ctx, t, acctest.ProviderMeta(ctx, t).IAMClient(ctx))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig_basic(groupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, t, resourceName, &conf),
					testAccCheckGroupState(resourceName, &conf, groupName),
					testAccCheckGroupAddExternalMember(ctx, t, fixture, resourceName),
				),
			},
			{
				Config:      testAccGroupConfig_empty(),
				ExpectError: regexache.MustCompile(`(?i)(DeleteGroup|group.*member|member.*group|cannot be deleted|DeleteConflict|Conflict)`),
			},
			{
				Config: testAccGroupConfig_basic(groupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupState(resourceName, &conf, groupName),
					testAccCheckGroupExternalMemberExists(ctx, t, fixture),
					testAccCheckGroupRemoveExternalMember(ctx, t, fixture),
				),
			},
		},
	})
}

type groupMemberFixture struct {
	groupName       string
	membershipAdded bool
	userCreated     bool
	userName        string
}

func testAccCreateGroupMemberFixture(ctx context.Context, t *testing.T, conn *iam.Client) *groupMemberFixture {
	t.Helper()

	fixture := &groupMemberFixture{
		userName: acctest.RandomWithPrefix(t, groupTestResourcePrefix+"-member"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyGroupMemberFixture(context.Background(), conn, fixture); err != nil {
			t.Errorf("destroying IAM Group member fixture: %s", err)
		}
	})

	_, err := conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		t.Fatalf("creating external IAM User (%s): %s", fixture.userName, err)
	}
	fixture.userCreated = true

	return fixture
}

func testAccDestroyGroupMemberFixture(ctx context.Context, conn *iam.Client, fixture *groupMemberFixture) error {
	if fixture == nil || !fixture.userCreated || fixture.userName == "" {
		return nil
	}

	if fixture.membershipAdded && fixture.groupName != "" {
		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(fixture.groupName),
			UserName:  aws.String(fixture.userName),
		})
		if err != nil {
			_, findErr := tfiam.FindGroupByName(ctx, conn, fixture.groupName)
			if !retry.NotFound(findErr) {
				return fmt.Errorf("removing external IAM User (%s) from Group (%s): %w", fixture.userName, fixture.groupName, err)
			}
		}
	}

	_, err := conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		_, findErr := tfiam.FindUserByName(ctx, conn, fixture.userName)
		if !retry.NotFound(findErr) {
			return fmt.Errorf("deleting external IAM User (%s): %w", fixture.userName, err)
		}
	}

	_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
		_, err := tfiam.FindUserByName(ctx, conn, fixture.userName)
		if retry.NotFound(err) {
			return nil, &retry.NotFoundError{LastError: err}
		}

		return nil, err
	})
	if err != nil {
		return fmt.Errorf("waiting for external IAM User (%s) deletion: %w", fixture.userName, err)
	}

	return nil
}

func testAccCheckGroupAddExternalMember(ctx context.Context, t *testing.T, fixture *groupMemberFixture, groupResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupRS, ok := s.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", groupResourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		groupName := groupRS.Primary.Attributes[names.AttrName]

		_, err := conn.AddUserToGroup(ctx, &iam.AddUserToGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(fixture.userName),
		})
		if err != nil {
			return fmt.Errorf("adding external IAM User (%s) to Group (%s): %w", fixture.userName, groupName, err)
		}
		fixture.groupName = groupName
		fixture.membershipAdded = true

		_, err = tfiam.FindGroupForUserByTwoPartKey(ctx, conn, fixture.userName, groupName)
		if err != nil {
			return fmt.Errorf("reading external IAM User Group Membership (%s/%s): %w", fixture.userName, groupName, err)
		}

		return nil
	}
}

func testAccCheckGroupRemoveExternalMember(ctx context.Context, t *testing.T, fixture *groupMemberFixture) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if !fixture.membershipAdded || fixture.groupName == "" {
			return fmt.Errorf("external IAM User (%s) is not bound to an IAM Group", fixture.userName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(fixture.groupName),
			UserName:  aws.String(fixture.userName),
		})
		if err != nil {
			return fmt.Errorf("removing external IAM User (%s) from Group (%s): %w", fixture.userName, fixture.groupName, err)
		}
		fixture.membershipAdded = false

		return nil
	}
}

func testAccCheckGroupExternalMemberExists(ctx context.Context, t *testing.T, fixture *groupMemberFixture) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if !fixture.membershipAdded || fixture.groupName == "" {
			return fmt.Errorf("external IAM User (%s) is not bound to an IAM Group", fixture.userName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, fixture.userName, fixture.groupName)
		if err != nil {
			return fmt.Errorf("reading external IAM User Group Membership (%s/%s): %w", fixture.userName, fixture.groupName, err)
		}

		return nil
	}
}

func testAccCheckGroupDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_group" {
				continue
			}

			_, err := tfiam.FindGroupByName(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Group %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckGroupExists(ctx context.Context, t *testing.T, n string, v *awstypes.Group) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindGroupByName(ctx, conn, rs.Primary.Attributes[names.AttrName])
		if err != nil {
			return err
		}

		*v = *output
		return nil
	}
}

func testAccCheckGroupDeleted(ctx context.Context, t *testing.T, groupName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := tfiam.FindGroupByName(ctx, conn, groupName)
			if retry.NotFound(err) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			return fmt.Errorf("waiting for IAM Group (%s) deletion: %w", groupName, err)
		}

		return nil
	}
}

func testAccCheckGroupRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM Group resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

func testAccCheckGroupDisappearsExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		groupName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.DeleteGroup(ctx, &iam.DeleteGroupInput{
			GroupName: aws.String(groupName),
		})
		if err != nil {
			return fmt.Errorf("externally deleting IAM Group (%s): %w", groupName, err)
		}

		return testAccCheckGroupDeleted(ctx, t, groupName)(s)
	}
}

func testAccCheckGroupState(resourceName string, group *awstypes.Group, wantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if got := aws.ToString(group.GroupName); got != wantName {
			return fmt.Errorf("bad remote group name: got %s, want %s", got, wantName)
		}

		if got := aws.ToString(group.Arn); got == "" {
			return fmt.Errorf("remote arn is empty")
		}

		if got := aws.ToString(group.GroupId); got == "" {
			return fmt.Errorf("remote group id is empty")
		}

		if got := rs.Primary.ID; got != aws.ToString(group.GroupName) {
			return fmt.Errorf("bad state id: got %s, want %s", got, aws.ToString(group.GroupName))
		}

		if got := rs.Primary.Attributes[names.AttrName]; got != wantName {
			return fmt.Errorf("bad state name: got %s, want %s", got, wantName)
		}

		if got := rs.Primary.Attributes[names.AttrName]; got != aws.ToString(group.GroupName) {
			return fmt.Errorf("state name %s does not match remote group name %s", got, aws.ToString(group.GroupName))
		}

		if got := rs.Primary.Attributes[names.AttrARN]; got != aws.ToString(group.Arn) {
			return fmt.Errorf("bad state arn: got %s, want %s", got, aws.ToString(group.Arn))
		}

		if got := rs.Primary.Attributes["unique_id"]; got != aws.ToString(group.GroupId) {
			return fmt.Errorf("bad state unique_id: got %s, want %s", got, aws.ToString(group.GroupId))
		}

		return nil
	}
}

func testAccGroupConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}
`, rName)
}

func testAccGroupConfig_empty() string {
	return `
terraform {}
`
}

func testAccGroupConfig_duplicateName(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test1" {
  name = %[1]q
}

resource "ctyun_iam_group" "test2" {
  name = %[1]q
}
`, rName)
}
