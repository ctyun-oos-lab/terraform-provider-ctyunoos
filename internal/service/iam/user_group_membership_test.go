// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"errors"
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
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	userGroupMembershipTestResourcePrefix    = acctest.ResourcePrefix + "-membership"
	userGroupMembershipLimitQuotaTestMaximum = 200
)

// TC-IAM-MEMBERSHIP-01-Basic Add verifies that after Terraform creates a user, a group, and a single membership, both the remote relationship and the state are correct.
func TestAccIAMUserGroupMembership_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var group awstypes.Group
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-user")
	groupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	resourceName := "ctyun_iam_user_group_membership.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipConfig_basic(userName, groupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName, groupName),
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
		},
	})
}

// TC-IAM-MEMBERSHIP-02-Input Validation verifies that referencing a non-existent user causes the server to reject the membership creation.
func TestAccIAMUserGroupMembership_userNotFound(t *testing.T) {
	ctx := acctest.Context(t)
	groupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	missingUserName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-missing-user")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipConfig_missingUser(groupName, missingUserName),
				ExpectError: regexache.MustCompile(`(?i)(NoSuchEntity|not found|does not exist|couldn't find\s+resource)`),
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-02-Input Validation verifies that referencing a non-existent group causes the server to reject the membership creation.
func TestAccIAMUserGroupMembership_groupNotFound(t *testing.T) {
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-user")
	missingGroupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-missing-group")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipConfig_missingGroup(userName, missingGroupName),
				ExpectError: regexache.MustCompile(`(?i)(NoSuchEntity|not found|does not exist|couldn't find\s+resource)`),
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-02-Input Validation verifies the local length and character-set validation of user and group.
func TestUnitIAMUserGroupMembership_ResourceUserGroupMembershipValidation(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceUserGroupMembership()

	userSchema, ok := resource.Schema["user"]
	if !ok || userSchema == nil {
		t.Fatal("expected user schema")
	}
	groupSchema, ok := resource.Schema["group"]
	if !ok || groupSchema == nil {
		t.Fatal("expected group schema")
	}

	userValidate := userSchema.ValidateFunc
	if userValidate == nil {
		t.Fatal("expected user validation")
	}

	groupValidate := groupSchema.ValidateFunc
	if groupValidate == nil {
		t.Fatal("expected group validation")
	}

	for _, tc := range []struct {
		name                         string
		required, optional, computed bool
		forceNew                     bool
	}{
		{
			name:     "user",
			required: userSchema.Required,
			optional: userSchema.Optional,
			computed: userSchema.Computed,
			forceNew: userSchema.ForceNew,
		},
		{
			name:     "group",
			required: groupSchema.Required,
			optional: groupSchema.Optional,
			computed: groupSchema.Computed,
			forceNew: groupSchema.ForceNew,
		},
	} {
		if !tc.forceNew {
			t.Errorf("expected %s changes to force a new membership", tc.name)
		}
		if !tc.required || tc.optional || tc.computed {
			t.Errorf("expected %s to be Required only; required=%t optional=%t computed=%t", tc.name, tc.required, tc.optional, tc.computed)
		}
	}

	userCases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid_characters", value: "User_-,.+@=123", wantErr: false},
		{name: "valid_min_length", value: "a", wantErr: false},
		{name: "valid_max_length", value: strings.Repeat("a", 64), wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 65), wantErr: true},
		{name: "contains_space", value: "invalid user", wantErr: true},
		{name: "contains_exclamation", value: "invalid!user", wantErr: true},
		{name: "contains_slash", value: "invalid/user", wantErr: true},
	}

	for _, tc := range userCases {
		t.Run("user_"+tc.name, func(t *testing.T) {
			_, errs := userValidate(tc.value, "user")
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; errors: %v", gotErr, tc.wantErr, errs)
			}
		})
	}

	groupCases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid_characters", value: "Group_-,.+@=123", wantErr: false},
		{name: "valid_min_length", value: "a", wantErr: false},
		{name: "valid_max_length", value: strings.Repeat("a", 128), wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 129), wantErr: true},
		{name: "contains_space", value: "invalid group", wantErr: true},
		{name: "contains_exclamation", value: "invalid!group", wantErr: true},
		{name: "contains_slash", value: "invalid/group", wantErr: true},
	}

	for _, tc := range groupCases {
		t.Run("group_"+tc.name, func(t *testing.T) {
			_, errs := groupValidate(tc.value, "group")
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; errors: %v", gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TC-IAM-MEMBERSHIP-02-Input Validation verifies that illegal values of user and group are rejected by the provider schema in the full Terraform flow.
func TestAccIAMUserGroupMembership_schemaValidation(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation("", "valid-group"),
				ExpectError: regexache.MustCompile(`expected length of user to be in the range \(1 - 64\), got`),
			},
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation(strings.Repeat("a", 65), "valid-group"),
				ExpectError: regexache.MustCompile(`expected length of user to be in the range \(1 - 64\), got`),
			},
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation("invalid/user", "valid-group"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation("valid-user", ""),
				ExpectError: regexache.MustCompile(`expected length of group to be in the range \(1 - 128\), got`),
			},
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation("valid-user", strings.Repeat("a", 129)),
				ExpectError: regexache.MustCompile(`expected length of group to be in the range \(1 - 128\), got`),
			},
			{
				Config:      testAccUserGroupMembershipConfig_schemaValidation("valid-user", "invalid/group"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-01-Basic Add additionally verifies that changing the ForceNew fields of user and group replaces the same membership address, cleaning up the old relationship and converging with no diff.
func TestAccIAMUserGroupMembership_replace(t *testing.T) {
	ctx := acctest.Context(t)
	var group awstypes.Group
	userName1 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-replace-user")
	userName2 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-replace-user")
	groupName1 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-replace-group")
	groupName2 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-replace-group")
	resourceName := "ctyun_iam_user_group_membership.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipConfig_replace(userName1, userName2, groupName1, groupName2, "user1", "group1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName1, groupName1),
				),
			},
			{
				Config: testAccUserGroupMembershipConfig_replace(userName1, userName2, groupName1, groupName2, "user1", "group2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName1, groupName2),
					testAccCheckUserGroupMembershipNotExists(ctx, t, userName1, groupName1),
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
			{
				Config: testAccUserGroupMembershipConfig_replace(userName1, userName2, groupName1, groupName2, "user2", "group2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName2, groupName2),
					testAccCheckUserGroupMembershipNotExists(ctx, t, userName1, groupName2),
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

// TC-IAM-MEMBERSHIP-03-Expand To Multiple Groups verifies that after adding a second membership, both target group relationships of the same user exist.
func TestAccIAMUserGroupMembership_expandToMultipleGroups(t *testing.T) {
	ctx := acctest.Context(t)
	var group1 awstypes.Group
	var group2 awstypes.Group
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-user")
	groupName1 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	groupName2 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	resourceName1 := "ctyun_iam_user_group_membership.group1"
	resourceName2 := "ctyun_iam_user_group_membership.group2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipConfig_oneGroup(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName1, &group1),
					testAccCheckUserGroupMembershipState(resourceName1, &group1, userName, groupName1),
				),
			},
			{
				Config: testAccUserGroupMembershipConfig_twoGroups(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName1, &group1),
					testAccCheckUserGroupMembershipState(resourceName1, &group1, userName, groupName1),
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName2, &group2),
					testAccCheckUserGroupMembershipState(resourceName2, &group2, userName, groupName2),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-04-Shrink Groups verifies that after removing one of two memberships, the retained relationship still exists and the extra relationship is deleted.
func TestAccIAMUserGroupMembership_shrinkGroups(t *testing.T) {
	ctx := acctest.Context(t)
	var group1 awstypes.Group
	var group2 awstypes.Group
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-user")
	groupName1 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	groupName2 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	resourceName1 := "ctyun_iam_user_group_membership.group1"
	resourceName2 := "ctyun_iam_user_group_membership.group2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipConfig_twoGroups(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName1, &group1),
					testAccCheckUserGroupMembershipState(resourceName1, &group1, userName, groupName1),
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName2, &group2),
					testAccCheckUserGroupMembershipState(resourceName2, &group2, userName, groupName2),
				),
			},
			{
				Config: testAccUserGroupMembershipConfig_oneGroup(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName1, &group1),
					testAccCheckUserGroupMembershipState(resourceName1, &group1, userName, groupName1),
					testAccCheckUserGroupMembershipNotExists(ctx, t, userName, groupName2),
					testAccCheckUserGroupMembershipRemovedFromState(resourceName2),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-05-Remove All verifies that after removing all membership configurations, every target group relationship of the same user is deleted.
func TestAccIAMUserGroupMembership_removeAll(t *testing.T) {
	ctx := acctest.Context(t)
	var group1 awstypes.Group
	var group2 awstypes.Group
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-user")
	groupName1 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	groupName2 := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-group")
	resourceName1 := "ctyun_iam_user_group_membership.group1"
	resourceName2 := "ctyun_iam_user_group_membership.group2"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipConfig_twoGroups(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName1, &group1),
					testAccCheckUserGroupMembershipState(resourceName1, &group1, userName, groupName1),
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName2, &group2),
					testAccCheckUserGroupMembershipState(resourceName2, &group2, userName, groupName2),
				),
			},
			{
				Config: testAccUserGroupMembershipConfig_noGroups(userName, groupName1, groupName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipNotExists(ctx, t, userName, groupName1),
					testAccCheckUserGroupMembershipNotExists(ctx, t, userName, groupName2),
					testAccCheckUserGroupMembershipRemovedFromState(resourceName1),
					testAccCheckUserGroupMembershipRemovedFromState(resourceName2),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-06-Repeated Or Limit Negative uses the SDK to bind a user up to the server-side membership limit, then verifies that creating the next membership with Terraform returns LimitExceeded.
func TestAccIAMUserGroupMembership_limitExceeded(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	membershipQuota := testAccPreCheckUserGroupMembershipLimitQuota(ctx, t, conn)
	fixture := testAccCreateUserGroupMembershipLimitFixture(ctx, t, conn, membershipQuota+1)
	groupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-limit-probe-group")

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipLimitExceededDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupMembershipConfig_limitExceeded(fixture.userName, groupName),
				ExpectError: regexache.MustCompile(`(?i)LimitExceeded`),
			},
		},
	})
}

// TC-IAM-MEMBERSHIP-07-Drift Recovery verifies that after the membership is removed remotely, re-applying the same declaration restores the relationship.
func TestAccIAMUserGroupMembership_driftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var group awstypes.Group
	userName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-drift-user")
	groupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-drift-group")
	resourceName := "ctyun_iam_user_group_membership.test"
	config := testAccUserGroupMembershipConfig_basic(userName, groupName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupMembershipDestroyWithDependencies(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName, groupName),
					testAccCheckUserGroupMembershipRemoveExternally(ctx, t, resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, t, resourceName, &group),
					testAccCheckUserGroupMembershipState(resourceName, &group, userName, groupName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCheckUserGroupMembershipExists(ctx context.Context, t *testing.T, resourceName string, groups ...*awstypes.Group) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, rs.Primary.Attributes["user"], rs.Primary.Attributes["group"])
		if err != nil {
			return err
		}

		if len(groups) > 0 && groups[0] != nil {
			*groups[0] = *output
		}
		return nil
	}
}

func testAccCheckUserGroupMembershipState(resourceName string, group *awstypes.Group, wantUser, wantGroup string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if got := aws.ToString(group.GroupName); got != wantGroup {
			return fmt.Errorf("remote group name = %q, want %q", got, wantGroup)
		}

		wantID := fmt.Sprintf("%s/%s", wantUser, wantGroup)
		if got := rs.Primary.ID; got != wantID {
			return fmt.Errorf("state id = %q, want %q", got, wantID)
		}

		if got := rs.Primary.Attributes["user"]; got != wantUser {
			return fmt.Errorf("state user = %q, want %q", got, wantUser)
		}

		if got := rs.Primary.Attributes["group"]; got != wantGroup {
			return fmt.Errorf("state group = %q, want %q", got, wantGroup)
		}

		return nil
	}
}

func testAccCheckUserGroupMembershipRemoveExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		userName := rs.Primary.Attributes["user"]
		groupName := rs.Primary.Attributes["group"]
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("externally removing IAM User (%s) from Group (%s): %w", userName, groupName, err)
		}

		_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, userName, groupName)
			if retry.NotFound(err) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			return fmt.Errorf("waiting for external removal of IAM User (%s) from Group (%s): %w", userName, groupName, err)
		}

		return nil
	}
}

func testAccCheckUserGroupMembershipNotExists(ctx context.Context, t *testing.T, user, group string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, user, group)
		if retry.NotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("IAM User Group Membership (%s/%s) still exists", user, group)
	}
}

func testAccCheckUserGroupMembershipRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM User Group Membership resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

func testAccCheckUserGroupMembershipDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for name, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user_group_membership" {
				continue
			}
			// The resource and the DS share the same TypeName; Destroy only checks the managed resource and skips data.*.
			if strings.HasPrefix(name, "data.") {
				continue
			}

			user := rs.Primary.Attributes["user"]
			group := rs.Primary.Attributes["group"]
			_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, user, group)
			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM User Group Membership (%s/%s) still exists", user, group)
		}

		return nil
	}
}

func testAccCheckUserGroupMembershipDestroyWithDependencies(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		testAccCheckUserGroupMembershipDestroy(ctx, t),
		testAccCheckUserDestroy(ctx, t),
		testAccCheckGroupDestroy(ctx, t),
	)
}

func testAccCheckUserGroupMembershipLimitExceededDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		testAccCheckUserGroupMembershipDestroy(ctx, t),
		testAccCheckGroupDestroy(ctx, t),
	)
}

func testAccUserGroupMembershipConfig_basic(userName, groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_group" "test" {
  name = %[2]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.test.name
}
`, userName, groupName)
}

func testAccUserGroupMembershipConfig_missingUser(groupName, missingUserName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = %[2]q
  group = ctyun_iam_group.test.name
}
`, groupName, missingUserName)
}

func testAccUserGroupMembershipConfig_missingGroup(userName, missingGroupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.test.name
  group = %[2]q
}
`, userName, missingGroupName)
}

func testAccUserGroupMembershipConfig_schemaValidation(userName, groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user_group_membership" "test" {
  user  = %[1]q
  group = %[2]q
}
`, userName, groupName)
}

func testAccUserGroupMembershipConfig_oneGroup(userName, groupName1, groupName2 string) string {
	return acctest.ConfigCompose(
		testAccUserGroupMembershipConfig_groups(userName, groupName1, groupName2),
		`
resource "ctyun_iam_user_group_membership" "group1" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.group1.name
}
`)
}

func testAccUserGroupMembershipConfig_twoGroups(userName, groupName1, groupName2 string) string {
	return acctest.ConfigCompose(
		testAccUserGroupMembershipConfig_groups(userName, groupName1, groupName2),
		`
resource "ctyun_iam_user_group_membership" "group1" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.group1.name
}

resource "ctyun_iam_user_group_membership" "group2" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.group2.name
}
`)
}

func testAccUserGroupMembershipConfig_noGroups(userName, groupName1, groupName2 string) string {
	return testAccUserGroupMembershipConfig_groups(userName, groupName1, groupName2)
}

func testAccUserGroupMembershipConfig_groups(userName, groupName1, groupName2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_group" "group1" {
  name = %[2]q
}

resource "ctyun_iam_group" "group2" {
  name = %[3]q
}
`, userName, groupName1, groupName2)
}

func testAccUserGroupMembershipConfig_replace(userName1, userName2, groupName1, groupName2, membershipUser, membershipGroup string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "user1" {
  name = %[1]q
}

resource "ctyun_iam_user" "user2" {
  name = %[2]q
}

resource "ctyun_iam_group" "group1" {
  name = %[3]q
}

resource "ctyun_iam_group" "group2" {
  name = %[4]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.%[5]s.name
  group = ctyun_iam_group.%[6]s.name
}
`, userName1, userName2, groupName1, groupName2, membershipUser, membershipGroup)
}

func testAccUserGroupMembershipConfig_limitExceeded(userName, groupName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = %[2]q
  group = ctyun_iam_group.test.name
}
`, groupName, userName)
}

type userGroupMembershipLimitFixture struct {
	userName        string
	userCreated     bool
	groupNames      []string
	boundGroupNames []string
}

func testAccPreCheckUserGroupMembershipLimitQuota(ctx context.Context, t *testing.T, conn *iam.Client) int {
	t.Helper()

	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	membershipQuota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroupsPerUserQuota)]
	if !ok {
		t.Fatalf("GroupsPerUserQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	if membershipQuota >= userGroupMembershipLimitQuotaTestMaximum {
		t.Skipf("GroupsPerUserQuota=%d is at least %d; skipping membership limit exhaustion test", membershipQuota, userGroupMembershipLimitQuotaTestMaximum)
	}

	groupsQuota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroupsQuota)]
	if !ok {
		t.Fatalf("GroupsQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	groupsUsed, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroups)]
	if !ok {
		t.Fatalf("Groups missing in SummaryMap: %#v", out.SummaryMap)
	}

	groupsNeeded := membershipQuota + 2 // Exhaustion probe plus the Terraform-managed probe group.
	groupsRemaining := groupsQuota - groupsUsed
	if groupsRemaining < groupsNeeded {
		t.Skipf("insufficient GroupsQuota remaining=%d need=%d to exhaust GroupsPerUserQuota=%d", groupsRemaining, groupsNeeded, membershipQuota)
	}

	return int(membershipQuota)
}

func testAccCreateUserGroupMembershipLimitFixture(ctx context.Context, t *testing.T, conn *iam.Client, maxGroups int) *userGroupMembershipLimitFixture {
	t.Helper()

	fixture := &userGroupMembershipLimitFixture{
		userName: acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-limit-user"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyUserGroupMembershipLimitFixture(context.Background(), conn, fixture); err != nil {
			t.Errorf("destroying IAM User Group Membership limit fixture: %s", err)
		}
	})

	_, err := conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		t.Fatalf("creating IAM User (%s) for membership limit fixture: %s", fixture.userName, err)
	}
	fixture.userCreated = true

	for range maxGroups {
		groupName := acctest.RandomWithPrefix(t, userGroupMembershipTestResourcePrefix+"-limit-group")

		_, err := conn.CreateGroup(ctx, &iam.CreateGroupInput{GroupName: aws.String(groupName)})
		if err != nil {
			t.Fatalf("creating IAM Group (%s) for membership limit fixture: %s", groupName, err)
		}
		fixture.groupNames = append(fixture.groupNames, groupName)

		_, err = conn.AddUserToGroup(ctx, &iam.AddUserToGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(fixture.userName),
		})
		if err == nil {
			fixture.boundGroupNames = append(fixture.boundGroupNames, groupName)
			continue
		}

		if tfawserr.ErrCodeEquals(err, "LimitExceeded") {
			return fixture
		}

		t.Fatalf("adding IAM User (%s) to Group (%s) while filling membership limit: %s", fixture.userName, groupName, err)
	}

	t.Fatalf("IAM User (%s) membership limit was not reached after %d groups", fixture.userName, maxGroups)
	return nil
}

func testAccDestroyUserGroupMembershipLimitFixture(ctx context.Context, conn *iam.Client, fixture *userGroupMembershipLimitFixture) error {
	if fixture == nil || !fixture.userCreated || fixture.userName == "" {
		return nil
	}

	var cleanupErrors []error

	for _, groupName := range fixture.boundGroupNames {
		_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
			GroupName: aws.String(groupName),
			UserName:  aws.String(fixture.userName),
		})
		if err != nil && !retry.NotFound(err) && !tfawserr.ErrHTTPStatusCodeEquals(err, 404) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("removing IAM User (%s) from Group (%s): %w", fixture.userName, groupName, err))
			continue
		}

		_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, fixture.userName, groupName)
			if retry.NotFound(err) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("waiting for IAM User (%s) removal from Group (%s): %w", fixture.userName, groupName, err))
		}
	}

	for _, groupName := range fixture.groupNames {
		_, err := conn.DeleteGroup(ctx, &iam.DeleteGroupInput{GroupName: aws.String(groupName)})
		if err != nil {
			if !retry.NotFound(err) && !tfawserr.ErrHTTPStatusCodeEquals(err, 404) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("deleting IAM Group (%s) from membership limit fixture: %w", groupName, err))
			}
			continue
		}

		_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := tfiam.FindGroupByName(ctx, conn, groupName)
			if retry.NotFound(err) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("waiting for IAM Group (%s) deletion: %w", groupName, err))
		}
	}

	_, err := conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		if !retry.NotFound(err) && !tfawserr.ErrHTTPStatusCodeEquals(err, 404) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("deleting IAM User (%s) from membership limit fixture: %w", fixture.userName, err))
		}
		return errors.Join(cleanupErrors...)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
		_, err := tfiam.FindUserByName(ctx, conn, fixture.userName)
		if retry.NotFound(err) {
			return nil, &retry.NotFoundError{LastError: err}
		}

		return nil, err
	})
	if err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("waiting for IAM User (%s) deletion: %w", fixture.userName, err))
	}

	return errors.Join(cleanupErrors...)
}
