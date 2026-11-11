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

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMPolicyEntitiesDataSource_schema verifies the schema shape of ctyun_iam_policy_entities and the validation of
// its arn input (covers the former TC-IAM-PEDS-04 ARN Validation Matrix):
// it contains arn/groups/roles/users; it does not contain upstream entity_filter/policy_arn etc.;
// the groups/roles/users elements are *_name/*_id only, without arn;
// only StringLenBetween(20, 2048); both valid policy ARNs and malformed-but-length-valid ARNs pass the schema; too-short/too-long values fail.
// The ARN format is validated by the server.
func TestUnitIAMPolicyEntitiesDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourcePolicyEntities()

	for _, name := range []string{names.AttrARN, "groups", "roles", "users"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	for _, name := range []string{"entity_filter", "policy_arn", "policy_groups", "policy_roles", "policy_users"} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("unexpected %s schema", name)
		}
	}

	arnValidate := dataSource.Schema[names.AttrARN].ValidateFunc
	if arnValidate == nil {
		t.Fatal("expected arn validation")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{
			name:  "valid_local",
			value: "arn:ctyun:iam::32fefj64y54gc:policy/test_policy",
		},
		{
			name:  "valid_oos",
			value: "arn:ctyun:iam::OOS:policy/IAMReadOnlyAccess",
		},
		{
			name:  "wrong_resource_role_passes_schema",
			value: "arn:ctyun:iam::32fefj64y54gc:role/test",
		},
		{
			name:  "wrong_resource_user_passes_schema",
			value: "arn:ctyun:iam::32fefj64y54gc:user/test",
		},
		{
			name:  "wrong_partition_aws_passes_schema",
			value: "arn:aws:iam::32fefj64y54gc:policy/test",
		},
		{
			name:  "malformed_missing_arn_prefix_passes_schema",
			value: "ctyun:iam::32fefj64y54gc:policy/test",
		},
		{
			name:  "malformed_passes_schema",
			value: "not-a-valid-policy-arn",
		},
		{
			name:  "too_short",
			value: "arn:ctyun:iam::1:p",
			errContains: []string{
				"expected length of arn to be in the range (20 - 2048)",
			},
		},
		{
			name:  "too_long",
			value: "arn:ctyun:iam::32fefj64y54gc:policy/" + strings.Repeat("a", 2048),
			errContains: []string{
				"expected length of arn to be in the range (20 - 2048)",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := arnValidate(tc.value, names.AttrARN)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for arn=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for arn=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for arn missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}

	groupSchema := dataSource.Schema["groups"].Elem.(*schema.Resource).Schema
	if _, ok := groupSchema[names.AttrARN]; ok {
		t.Fatalf("unexpected groups.arn schema")
	}
	if _, ok := groupSchema["group_name"]; !ok {
		t.Fatalf("expected groups.group_name schema")
	}
	if _, ok := groupSchema["group_id"]; !ok {
		t.Fatalf("expected groups.group_id schema")
	}

	roleSchema := dataSource.Schema["roles"].Elem.(*schema.Resource).Schema
	if _, ok := roleSchema[names.AttrARN]; ok {
		t.Fatalf("unexpected roles.arn schema")
	}
	if _, ok := roleSchema["role_name"]; !ok {
		t.Fatalf("expected roles.role_name schema")
	}

	userSchema := dataSource.Schema["users"].Elem.(*schema.Resource).Schema
	if _, ok := userSchema[names.AttrARN]; ok {
		t.Fatalf("unexpected users.arn schema")
	}
	if _, ok := userSchema["user_name"]; !ok {
		t.Fatalf("expected users.user_name schema")
	}
}

// TestUnitIAMPolicyEntitiesDataSource_flattenPolicyEntities verifies flattenPolicyGroups/Roles/Users:
// empty input returns a non-nil empty slice; Name/Id mapping and list order are correct for single/multiple entities
// (replaces the entity ARN derivation assertions of the former TC-IAM-PEDS-05; list items are now *_name/*_id).
func TestUnitIAMPolicyEntitiesDataSource_flattenPolicyEntities(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		groups := tfiam.FlattenPolicyGroups(nil)
		if groups == nil {
			t.Fatal("expected non-nil empty groups slice for nil input")
		}
		if got, want := len(groups), 0; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		groups = tfiam.FlattenPolicyGroups([]awstypes.PolicyGroup{})
		if groups == nil {
			t.Fatal("expected non-nil empty groups slice for empty input")
		}
		if got, want := len(groups), 0; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}

		roles := tfiam.FlattenPolicyRoles(nil)
		if roles == nil {
			t.Fatal("expected non-nil empty roles slice for nil input")
		}
		if got, want := len(roles), 0; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		roles = tfiam.FlattenPolicyRoles([]awstypes.PolicyRole{})
		if roles == nil {
			t.Fatal("expected non-nil empty roles slice for empty input")
		}
		if got, want := len(roles), 0; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}

		users := tfiam.FlattenPolicyUsers(nil)
		if users == nil {
			t.Fatal("expected non-nil empty users slice for nil input")
		}
		if got, want := len(users), 0; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		users = tfiam.FlattenPolicyUsers([]awstypes.PolicyUser{})
		if users == nil {
			t.Fatal("expected non-nil empty users slice for empty input")
		}
		if got, want := len(users), 0; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
	})

	t.Run("one", func(t *testing.T) {
		t.Parallel()

		groups := tfiam.FlattenPolicyGroups([]awstypes.PolicyGroup{
			{
				GroupId:   aws.String("AIDA6DQOBIAD7BL4YN5OA"),
				GroupName: aws.String("11111333"),
			},
		})
		if got, want := len(groups), 1; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if got, want := groups[0]["group_name"], "11111333"; got != want {
			t.Fatalf("expected group_name %v, got %v", want, got)
		}
		if got, want := groups[0]["group_id"], "AIDA6DQOBIAD7BL4YN5OA"; got != want {
			t.Fatalf("expected group_id %v, got %v", want, got)
		}

		roles := tfiam.FlattenPolicyRoles([]awstypes.PolicyRole{
			{
				RoleId:   aws.String("AIDA6DQOBIAD7BL4YN5O1"),
				RoleName: aws.String("11111333"),
			},
		})
		if got, want := len(roles), 1; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		if got, want := roles[0]["role_name"], "11111333"; got != want {
			t.Fatalf("expected role_name %v, got %v", want, got)
		}
		if got, want := roles[0]["role_id"], "AIDA6DQOBIAD7BL4YN5O1"; got != want {
			t.Fatalf("expected role_id %v, got %v", want, got)
		}

		users := tfiam.FlattenPolicyUsers([]awstypes.PolicyUser{
			{
				UserId:   aws.String("AIDA6DQOBIAD7BL4YN5OB"),
				UserName: aws.String("11111333"),
			},
		})
		if got, want := len(users), 1; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		if got, want := users[0]["user_name"], "11111333"; got != want {
			t.Fatalf("expected user_name %v, got %v", want, got)
		}
		if got, want := users[0]["user_id"], "AIDA6DQOBIAD7BL4YN5OB"; got != want {
			t.Fatalf("expected user_id %v, got %v", want, got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()

		groups := tfiam.FlattenPolicyGroups([]awstypes.PolicyGroup{
			{GroupId: aws.String("gid-1"), GroupName: aws.String("group-a")},
			{GroupId: aws.String("gid-2"), GroupName: aws.String("group-b")},
		})
		if got, want := len(groups), 2; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if got, want := groups[0]["group_name"], "group-a"; got != want {
			t.Fatalf("expected groups[0].group_name %v, got %v", want, got)
		}
		if got, want := groups[0]["group_id"], "gid-1"; got != want {
			t.Fatalf("expected groups[0].group_id %v, got %v", want, got)
		}
		if got, want := groups[1]["group_name"], "group-b"; got != want {
			t.Fatalf("expected groups[1].group_name %v, got %v", want, got)
		}
		if got, want := groups[1]["group_id"], "gid-2"; got != want {
			t.Fatalf("expected groups[1].group_id %v, got %v", want, got)
		}

		roles := tfiam.FlattenPolicyRoles([]awstypes.PolicyRole{
			{RoleId: aws.String("rid-1"), RoleName: aws.String("role-a")},
			{RoleId: aws.String("rid-2"), RoleName: aws.String("role-b")},
		})
		if got, want := len(roles), 2; got != want {
			t.Fatalf("expected %d roles, got %d", want, got)
		}
		if got, want := roles[0]["role_name"], "role-a"; got != want {
			t.Fatalf("expected roles[0].role_name %v, got %v", want, got)
		}
		if got, want := roles[0]["role_id"], "rid-1"; got != want {
			t.Fatalf("expected roles[0].role_id %v, got %v", want, got)
		}
		if got, want := roles[1]["role_name"], "role-b"; got != want {
			t.Fatalf("expected roles[1].role_name %v, got %v", want, got)
		}
		if got, want := roles[1]["role_id"], "rid-2"; got != want {
			t.Fatalf("expected roles[1].role_id %v, got %v", want, got)
		}

		users := tfiam.FlattenPolicyUsers([]awstypes.PolicyUser{
			{UserId: aws.String("uid-1"), UserName: aws.String("user-a")},
			{UserId: aws.String("uid-2"), UserName: aws.String("user-b")},
		})
		if got, want := len(users), 2; got != want {
			t.Fatalf("expected %d users, got %d", want, got)
		}
		if got, want := users[0]["user_name"], "user-a"; got != want {
			t.Fatalf("expected users[0].user_name %v, got %v", want, got)
		}
		if got, want := users[0]["user_id"], "uid-1"; got != want {
			t.Fatalf("expected users[0].user_id %v, got %v", want, got)
		}
		if got, want := users[1]["user_name"], "user-b"; got != want {
			t.Fatalf("expected users[1].user_name %v, got %v", want, got)
		}
		if got, want := users[1]["user_id"], "uid-2"; got != want {
			t.Fatalf("expected users[1].user_id %v, got %v", want, got)
		}
	})
}

// TC-IAM-PEDS-01-Basic All Entity Types verifies creating a policy and attaching it to one user, one group, and one role,
// then reading: the DS state fields id/arn/groups/roles/users (including *_name/*_id) match the cloud
// ListEntitiesForPolicy result (not compared against managed resource state); it also explicitly asserts that the
// groups/roles/users counts are all 1; CheckDestroy at the end confirms the fixtures are destroyed.
func TestAccIAMPolicyEntitiesDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policy_entities.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyEntitiesDataSourceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyEntitiesDataSourceConfig_all(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "1"),
				),
			},
		},
	})
}

// TC-IAM-PEDS-02-Empty Result verifies reading a policy attached to no entity: the DS state matches the empty cloud
// ListEntitiesForPolicy result, with id/arn equal to the input policy ARN; it also explicitly asserts that the
// groups/roles/users counts are all 0; CheckDestroy at the end.
func TestAccIAMPolicyEntitiesDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policy_entities.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyEntitiesDataSourceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyEntitiesDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-PEDS-03-Partial Entity Matrix verifies six partial-attachment configurations (group only / role only / user only /
// group+user / group+role / user+role): for each, the DS state matches the cloud ListEntitiesForPolicy result (not compared
// against managed resource state), explicitly asserting that attached types have count 1 and unattached types count 0;
// CheckDestroy at the end.
func TestAccIAMPolicyEntitiesDataSource_partialMatrix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	testCases := map[string]struct {
		config     func(string) string
		wantGroups string
		wantRoles  string
		wantUsers  string
	}{
		"group_only": {
			config:     testAccPolicyEntitiesDataSourceConfig_groupOnly,
			wantGroups: "1",
			wantRoles:  "0",
			wantUsers:  "0",
		},
		"role_only": {
			config:     testAccPolicyEntitiesDataSourceConfig_roleOnly,
			wantGroups: "0",
			wantRoles:  "1",
			wantUsers:  "0",
		},
		"user_only": {
			config:     testAccPolicyEntitiesDataSourceConfig_userOnly,
			wantGroups: "0",
			wantRoles:  "0",
			wantUsers:  "1",
		},
		"group_and_user": {
			config:     testAccPolicyEntitiesDataSourceConfig_groupAndUser,
			wantGroups: "1",
			wantRoles:  "0",
			wantUsers:  "1",
		},
		"group_and_role": {
			config:     testAccPolicyEntitiesDataSourceConfig_groupAndRole,
			wantGroups: "1",
			wantRoles:  "1",
			wantUsers:  "0",
		},
		"user_and_role": {
			config:     testAccPolicyEntitiesDataSourceConfig_userAndRole,
			wantGroups: "0",
			wantRoles:  "1",
			wantUsers:  "1",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dataSourceName := "data.ctyun_iam_policy_entities.test"

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				CheckDestroy:             testAccCheckPolicyEntitiesDataSourceDestroy(ctx, t),
				Steps: []resource.TestStep{
					{
						Config: tc.config(rName + "-" + name),
						Check: resource.ComposeAggregateTestCheckFunc(
							testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
							resource.TestCheckResourceAttr(dataSourceName, "groups.#", tc.wantGroups),
							resource.TestCheckResourceAttr(dataSourceName, "roles.#", tc.wantRoles),
							resource.TestCheckResourceAttr(dataSourceName, "users.#", tc.wantUsers),
						),
					},
				},
			})
		})
	}
}

// TC-IAM-PEDS-04-Invalid ARN Negative verifies that a malformed-but-length-valid arn passes the length validation and
// fails at the ListEntitiesForPolicy read stage with a parameter validation error rather than NotFound.
// The schema allows this ARN; the server is expected to return "ARN not-a-valid-policy-arn is not valid".
func TestAccIAMPolicyEntitiesDataSource_invalidARN(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyEntitiesDataSourceConfig_arnOnly("not-a-valid-policy-arn"),
				ExpectError: regexache.MustCompile(`(?s)ARN not-a-valid-policy-arn is not valid`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccPolicyEntitiesDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckPolicyEntitiesDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-PEDS-06-NotFound Negative verifies that reading a non-existent policy ARN returns a clear NotFound-style error,
// and that no data.ctyun_iam_policy_entities remains in state (no half-written result).
//
// Server-behavior dependency: findEntitiesForPolicy passes ListEntitiesForPolicy errors through unchanged; the provider does
// not normalize NoSuchEntity or an empty list into fixed semantics. This case's ExpectError assumes the current OOS returns
// an error (NoSuchEntity / 404, etc.) for a non-existent policy ARN rather than empty groups/roles/users.
func TestAccIAMPolicyEntitiesDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	missingName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingARN := fmt.Sprintf("arn:ctyun:iam::32fefj64y54gc:policy/%s", missingName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyEntitiesDataSourceConfig_arnOnly(missingARN),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccPolicyEntitiesDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckPolicyEntitiesDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-PEDS-06-Other Account Negative verifies that reading a policy that exists under another account returns AccessDenied,
// and that no data.ctyun_iam_policy_entities remains in state (no half-written result).
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the policy with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMPolicyEntitiesDataSource_otherAccount(t *testing.T) {
	policyARN := testAccCrossAccountPolicyARN(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyEntitiesDataSourceConfig_otherAccountPolicy(policyARN),
				ExpectError: regexache.MustCompile(`(?s)(AccessDenied|StatusCode: 403)`),
			},
			{
				Config:             testAccPolicyEntitiesDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckPolicyEntitiesDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-PEDS-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state matches
// the cloud ListEntitiesForPolicy result with no cross-leakage between entity types; the second read-back shows no drift
// relative to the first snapshot for id/arn/groups/roles/users; CheckDestroy at the end.
func TestAccIAMPolicyEntitiesDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policy_entities.test"
	var snapshot testAccPolicyEntitiesDataSourceSnapshot

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyEntitiesDataSourceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyEntitiesDataSourceConfig_all(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckPolicyEntitiesNoCrossLeak(dataSourceName),
					testAccCheckPolicyEntitiesDataSourceCaptureSnapshot(dataSourceName, &snapshot),
				),
			},
			{
				Config: testAccPolicyEntitiesDataSourceConfig_all(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckPolicyEntitiesNoCrossLeak(dataSourceName),
					testAccCheckPolicyEntitiesDataSourceMatchesSnapshot(dataSourceName, &snapshot),
				),
			},
		},
	})
}

// ListEntitiesForPolicy documentation: MaxItems defaults to 100; beyond that IsTruncated=true and a Marker is required to continue.
// The DS Read cannot change MaxItems, so this case attaches more users than the default page size to the same policy to force
// the real Read through multi-page aggregation.
const testAccIAMPolicyEntitiesListDefaultMaxItems = 100

// TC-IAM-PEDS-09-Pagination verifies that when the number of entities attached to the same policy exceeds the
// ListEntitiesForPolicy default MaxItems, the data source Read (PolicyArn only → default pagination) still returns all attached users.
func TestAccIAMPolicyEntitiesDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	entityCount := testAccIAMPolicyEntitiesListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policy_entities.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckPolicyEntitiesPaginationUsersQuota(ctx, t, entityCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyEntitiesDataSourceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyEntitiesDataSourceConfig_pagination(rName, entityCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPolicyEntitiesListDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "roles.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "users.#", strconv.Itoa(entityCount)),
				),
			},
		},
	})
}

// testAccCheckPolicyEntitiesDataSourceStateMatchesCloud checks that the DS state matches the cloud ListEntitiesForPolicy
// result without comparing against managed resource state: id/arn equal the queried policy ARN (and match GetPolicy), and the
// groups/roles/users sets match the cloud exactly (no more, no less) with consistent *_name/*_id fields.
func testAccCheckPolicyEntitiesDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		attrs := rs.Primary.Attributes
		policyARN := attrs[names.AttrARN]
		if policyARN == "" {
			return fmt.Errorf("bad arn: empty")
		}
		if rs.Primary.ID != policyARN {
			return fmt.Errorf("bad id: got %s, want %s (same as arn)", rs.Primary.ID, policyARN)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudPolicy, err := tfiam.FindPolicyByARN(ctx, conn, policyARN)
		if err != nil {
			return fmt.Errorf("reading IAM Policy (%s): %w", policyARN, err)
		}
		if cloudPolicy == nil {
			return fmt.Errorf("IAM Policy (%s) not found in cloud", policyARN)
		}
		if got, want := policyARN, aws.ToString(cloudPolicy.Arn); got != want {
			return fmt.Errorf("bad arn vs cloud GetPolicy: got %s, want %s", got, want)
		}

		groups, roles, users, err := tfiam.FindEntitiesForPolicy(ctx, conn, &iam.ListEntitiesForPolicyInput{
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("listing IAM Policy Entities (%s): %w", policyARN, err)
		}

		if err := matchPolicyEntityList(attrs, "groups", "group_name", "group_id", cloudGroupPairs(groups)); err != nil {
			return err
		}
		if err := matchPolicyEntityList(attrs, "roles", "role_name", "role_id", cloudRolePairs(roles)); err != nil {
			return err
		}
		if err := matchPolicyEntityList(attrs, "users", "user_name", "user_id", cloudUserPairs(users)); err != nil {
			return err
		}

		return nil
	}
}

type policyEntityPair struct {
	name string
	id   string
}

func cloudGroupPairs(groups []awstypes.PolicyGroup) []policyEntityPair {
	out := make([]policyEntityPair, 0, len(groups))
	for _, g := range groups {
		out = append(out, policyEntityPair{name: aws.ToString(g.GroupName), id: aws.ToString(g.GroupId)})
	}
	return out
}

func cloudRolePairs(roles []awstypes.PolicyRole) []policyEntityPair {
	out := make([]policyEntityPair, 0, len(roles))
	for _, r := range roles {
		out = append(out, policyEntityPair{name: aws.ToString(r.RoleName), id: aws.ToString(r.RoleId)})
	}
	return out
}

func cloudUserPairs(users []awstypes.PolicyUser) []policyEntityPair {
	out := make([]policyEntityPair, 0, len(users))
	for _, u := range users {
		out = append(out, policyEntityPair{name: aws.ToString(u.UserName), id: aws.ToString(u.UserId)})
	}
	return out
}

func matchPolicyEntityList(attrs map[string]string, listAttr, nameAttr, idAttr string, cloud []policyEntityPair) error {
	stateCount, err := strconv.Atoi(attrs[listAttr+".#"])
	if err != nil {
		return fmt.Errorf("parsing %s.#: %w", listAttr, err)
	}
	if stateCount != len(cloud) {
		return fmt.Errorf("%s count mismatch: state=%d listEntitiesForPolicy=%d", listAttr, stateCount, len(cloud))
	}

	cloudByID := make(map[string]string, len(cloud))
	for _, e := range cloud {
		if e.id == "" {
			return fmt.Errorf("ListEntitiesForPolicy returned empty %s", idAttr)
		}
		if _, dup := cloudByID[e.id]; dup {
			return fmt.Errorf("duplicate %s %q in ListEntitiesForPolicy", idAttr, e.id)
		}
		cloudByID[e.id] = e.name
	}

	seen := make(map[string]struct{}, stateCount)
	for i := 0; i < stateCount; i++ {
		prefix := fmt.Sprintf("%s.%d.", listAttr, i)
		stateID := attrs[prefix+idAttr]
		stateName := attrs[prefix+nameAttr]
		if stateID == "" {
			return fmt.Errorf("empty %s.%d.%s", listAttr, i, idAttr)
		}
		if _, dup := seen[stateID]; dup {
			return fmt.Errorf("duplicate %s.%d.%s %q in state", listAttr, i, idAttr, stateID)
		}
		seen[stateID] = struct{}{}

		wantName, ok := cloudByID[stateID]
		if !ok {
			return fmt.Errorf("%s.%d.%s %q not found in ListEntitiesForPolicy", listAttr, i, idAttr, stateID)
		}
		if stateName != wantName {
			return fmt.Errorf("bad %s.%d.%s: got %s, want %s", listAttr, i, nameAttr, stateName, wantName)
		}
	}

	if len(seen) != len(cloudByID) {
		return fmt.Errorf("%s id set size mismatch after field checks: state=%d cloud=%d", listAttr, len(seen), len(cloudByID))
	}
	return nil
}

// testAccCheckPolicyEntitiesDataSourceDestroy checks that every attachment/user/group/role/policy this DS fixture may have created is destroyed.
func testAccCheckPolicyEntitiesDataSourceDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckUserPolicyAttachmentDestroy(ctx, t),
		testAccCheckGroupPolicyAttachmentDestroy(ctx, t),
		testAccCheckRolePolicyAttachmentDestroy(ctx, t),
		testAccCheckUserDestroy(ctx, t),
		testAccCheckGroupDestroy(ctx, t),
		testAccCheckRoleDestroy(ctx, t),
		testAccCheckPolicyDestroy(ctx, t),
	)
}

// testAccPreCheckPolicyEntitiesPaginationUsersQuota confirms that the remaining UsersQuota is sufficient to create pageSize+1 users.
func testAccPreCheckPolicyEntitiesPaginationUsersQuota(ctx context.Context, t *testing.T, need int) {
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

// testAccCheckPolicyEntitiesListDefaultPageTruncated fetches one page (without paging further) using the same PolicyArn-only
// Input as the DS Read, confirming the policy has more attached entities than the default MaxItems with IsTruncated=true
// (otherwise this case has not actually exercised the DS pagination path).
func testAccCheckPolicyEntitiesListDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		policyARN := rs.Primary.Attributes[names.AttrARN]
		if policyARN == "" {
			return fmt.Errorf("bad arn: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListEntitiesForPolicy(ctx, &iam.ListEntitiesForPolicyInput{
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return fmt.Errorf("ListEntitiesForPolicy(default MaxItems) single page: %w", err)
		}
		firstPageCount := len(page.PolicyGroups) + len(page.PolicyRoles) + len(page.PolicyUsers)
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListEntitiesForPolicy page (MaxItems default %d); first page returned %d entities — DS Read would not exercise pagination", testAccIAMPolicyEntitiesListDefaultMaxItems, firstPageCount)
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

// testAccPolicyEntitiesDataSourceSnapshot saves the fields read back by the DS for drift comparison across two consecutive
// applies. groups/roles/users are stored as id→name sets so that list order changes are not misreported as drift.
type testAccPolicyEntitiesDataSourceSnapshot struct {
	id     string
	arn    string
	groups map[string]string
	roles  map[string]string
	users  map[string]string
}

func testAccCheckPolicyEntitiesDataSourceCaptureSnapshot(dataSourceName string, snapshot *testAccPolicyEntitiesDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		groups, err := collectPolicyEntityPairs(rs.Primary.Attributes, "groups", "group_name", "group_id")
		if err != nil {
			return err
		}
		roles, err := collectPolicyEntityPairs(rs.Primary.Attributes, "roles", "role_name", "role_id")
		if err != nil {
			return err
		}
		users, err := collectPolicyEntityPairs(rs.Primary.Attributes, "users", "user_name", "user_id")
		if err != nil {
			return err
		}

		*snapshot = testAccPolicyEntitiesDataSourceSnapshot{
			id:     rs.Primary.ID,
			arn:    rs.Primary.Attributes[names.AttrARN],
			groups: groups,
			roles:  roles,
			users:  users,
		}
		return nil
	}
}

func testAccCheckPolicyEntitiesDataSourceMatchesSnapshot(dataSourceName string, snapshot *testAccPolicyEntitiesDataSourceSnapshot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if snapshot == nil || snapshot.id == "" {
			return fmt.Errorf("policy entities data source snapshot is empty")
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

		groups, err := collectPolicyEntityPairs(rs.Primary.Attributes, "groups", "group_name", "group_id")
		if err != nil {
			return err
		}
		roles, err := collectPolicyEntityPairs(rs.Primary.Attributes, "roles", "role_name", "role_id")
		if err != nil {
			return err
		}
		users, err := collectPolicyEntityPairs(rs.Primary.Attributes, "users", "user_name", "user_id")
		if err != nil {
			return err
		}

		if err := matchStringMapExact("groups", groups, snapshot.groups); err != nil {
			return err
		}
		if err := matchStringMapExact("roles", roles, snapshot.roles); err != nil {
			return err
		}
		if err := matchStringMapExact("users", users, snapshot.users); err != nil {
			return err
		}

		return nil
	}
}

func collectPolicyEntityPairs(attrs map[string]string, listAttr, nameAttr, idAttr string) (map[string]string, error) {
	count, err := strconv.Atoi(attrs[listAttr+".#"])
	if err != nil {
		return nil, fmt.Errorf("parsing %s.#: %w", listAttr, err)
	}
	out := make(map[string]string, count)
	for i := 0; i < count; i++ {
		prefix := fmt.Sprintf("%s.%d.", listAttr, i)
		id := attrs[prefix+idAttr]
		name := attrs[prefix+nameAttr]
		if id == "" {
			return nil, fmt.Errorf("empty %s.%d.%s", listAttr, i, idAttr)
		}
		out[id] = name
	}
	return out, nil
}

func matchStringMapExact(label string, got, want map[string]string) error {
	if len(got) != len(want) {
		return fmt.Errorf("drift %s count: got %d, want %d", label, len(got), len(want))
	}
	for id, wantName := range want {
		gotName, ok := got[id]
		if !ok {
			return fmt.Errorf("drift %s missing id %q", label, id)
		}
		if gotName != wantName {
			return fmt.Errorf("drift %s[%s]: got %s, want %s", label, id, gotName, wantName)
		}
	}
	for id := range got {
		if _, ok := want[id]; !ok {
			return fmt.Errorf("drift %s has extra id %q", label, id)
		}
	}
	return nil
}

func testAccCheckPolicyEntitiesDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_policy_entities.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q groups.#=%q roles.#=%q users.#=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes["groups.#"],
				rs.Primary.Attributes["roles.#"],
				rs.Primary.Attributes["users.#"],
			)
		}
		return nil
	}
}

func testAccCheckPolicyEntitiesNoCrossLeak(dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		nameAttr := map[string]string{
			"groups": "group_name",
			"roles":  "role_name",
			"users":  "user_name",
		}
		idAttr := map[string]string{
			"groups": "group_id",
			"roles":  "role_id",
			"users":  "user_id",
		}

		for _, listAttr := range []string{"groups", "roles", "users"} {
			countStr := rs.Primary.Attributes[listAttr+".#"]
			count, err := strconv.Atoi(countStr)
			if err != nil {
				return fmt.Errorf("invalid %s.#: %w", listAttr, err)
			}
			for i := 0; i < count; i++ {
				nameKey := fmt.Sprintf("%s.%d.%s", listAttr, i, nameAttr[listAttr])
				idKey := fmt.Sprintf("%s.%d.%s", listAttr, i, idAttr[listAttr])
				if rs.Primary.Attributes[nameKey] == "" {
					return fmt.Errorf("%s is empty", nameKey)
				}
				if rs.Primary.Attributes[idKey] == "" {
					return fmt.Errorf("%s is empty", idKey)
				}
				// No ARN fields should remain
				if _, ok := rs.Primary.Attributes[fmt.Sprintf("%s.%d.arn", listAttr, i)]; ok {
					return fmt.Errorf("%s.%d.arn should not be present", listAttr, i)
				}
			}
		}

		return nil
	}
}

func testAccPolicyEntitiesDataSourceConfig_policy(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "policy entities test"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}
`, rName)
}

func testAccPolicyEntitiesDataSourceConfig_user(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}
`, rName)
}

func testAccPolicyEntitiesDataSourceConfig_group(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}
`, rName)
}

func testAccPolicyEntitiesDataSourceConfig_role(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = %[2]s
}
`, rName, testAccRoleAssumePolicyHCL("ec2.amazonaws.com"))
}

func testAccPolicyEntitiesDataSourceConfig_data() string {
	return `
data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn
}
`
}

func testAccPolicyEntitiesDataSourceConfig_empty(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_data(),
	)
}

func testAccPolicyEntitiesDataSourceConfig_all(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_user(rName),
		testAccPolicyEntitiesDataSourceConfig_group(rName),
		testAccPolicyEntitiesDataSourceConfig_role(rName),
		`
resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [
    ctyun_iam_user_policy_attachment.test,
    ctyun_iam_group_policy_attachment.test,
    ctyun_iam_role_policy_attachment.test,
  ]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_groupOnly(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_group(rName),
		`
resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [ctyun_iam_group_policy_attachment.test]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_roleOnly(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_role(rName),
		`
resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [ctyun_iam_role_policy_attachment.test]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_userOnly(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_user(rName),
		`
resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_groupAndUser(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_user(rName),
		testAccPolicyEntitiesDataSourceConfig_group(rName),
		`
resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [
    ctyun_iam_user_policy_attachment.test,
    ctyun_iam_group_policy_attachment.test,
  ]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_groupAndRole(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_group(rName),
		testAccPolicyEntitiesDataSourceConfig_role(rName),
		`
resource "ctyun_iam_group_policy_attachment" "test" {
  group      = ctyun_iam_group.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [
    ctyun_iam_group_policy_attachment.test,
    ctyun_iam_role_policy_attachment.test,
  ]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_userAndRole(rName string) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		testAccPolicyEntitiesDataSourceConfig_user(rName),
		testAccPolicyEntitiesDataSourceConfig_role(rName),
		`
resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

resource "ctyun_iam_role_policy_attachment" "test" {
  role       = ctyun_iam_role.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [
    ctyun_iam_user_policy_attachment.test,
    ctyun_iam_role_policy_attachment.test,
  ]
}
`,
	)
}

func testAccPolicyEntitiesDataSourceConfig_arnOnly(policyARN string) string {
	return fmt.Sprintf(`
data "ctyun_iam_policy_entities" "test" {
  arn = %[1]q
}
`, policyARN)
}

func testAccPolicyEntitiesDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed policy_entities read left no residual state
`
}

func testAccPolicyEntitiesDataSourceConfig_otherAccountPolicy(policyARN string) string {
	return fmt.Sprintf(`
# The cross-account policy is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_policy_entities" "test" {
  arn = %[1]q
}
`, policyARN)
}

func testAccPolicyEntitiesDataSourceConfig_pagination(rName string, userCount int) string {
	return acctest.ConfigCompose(
		testAccPolicyEntitiesDataSourceConfig_policy(rName),
		fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  count = %[2]d
  name  = "%[1]s-${count.index}"
}

resource "ctyun_iam_user_policy_attachment" "test" {
  count      = %[2]d
  user       = ctyun_iam_user.test[count.index].name
  policy_arn = ctyun_iam_policy.test.arn
}

data "ctyun_iam_policy_entities" "test" {
  arn = ctyun_iam_policy.test.arn

  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`, rName, userCount),
	)
}
