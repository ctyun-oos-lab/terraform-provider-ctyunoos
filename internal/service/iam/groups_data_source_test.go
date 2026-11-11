// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
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

// TestUnitIAMGroupsDataSource_schema verifies the schema shape of ctyun_iam_groups:
// it contains arns/names/name_regex/groups; it does not contain the upstream single-resource group_name;
// the groups elements contain arn/create_date/group_id/group_name/policies/users (policies/users are TypeInt);
// name_regex has a ValidateFunc.
func TestUnitIAMGroupsDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceGroups()

	for _, name := range []string{names.AttrARNs, "groups", "name_regex", names.AttrNames} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	if _, ok := dataSource.Schema[names.AttrGroupName]; ok {
		t.Fatalf("unexpected group_name schema")
	}

	groupSchema := dataSource.Schema["groups"].Elem.(*schema.Resource).Schema
	for _, name := range []string{
		names.AttrARN,
		"create_date",
		"group_id",
		names.AttrGroupName,
		"policies",
		"users",
	} {
		if _, ok := groupSchema[name]; !ok {
			t.Fatalf("expected groups.%s schema", name)
		}
	}

	if got := groupSchema["policies"].Type; got != schema.TypeInt {
		t.Fatalf("expected policies to be TypeInt, got %v", got)
	}
	if got := groupSchema["users"].Type; got != schema.TypeInt {
		t.Fatalf("expected users to be TypeInt, got %v", got)
	}

	if dataSource.Schema["name_regex"].ValidateFunc == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}
}

// TestUnitIAMGroupsDataSource_nameRegexValidation verifies the schema ValidateFunc of name_regex:
// valid regular expressions pass; various invalid regex forms return clear parsing errors (covers the former TC-IAM-DGROUPS-04).
func TestUnitIAMGroupsDataSource_nameRegexValidation(t *testing.T) {
	t.Parallel()

	validate := tfiam.DataSourceGroups().Schema["name_regex"].ValidateFunc
	if validate == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_prefix", value: "^tf-", errContains: nil},
		{name: "valid_exact", value: `^my-group-123$`, errContains: nil},
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

// TestUnitIAMGroupsDataSource_flattenGroups verifies flattenGroups:
// the normal path maps arn/create_date/group_id/group_name and the metadata policies/users;
// empty input returns a non-nil empty groups slice with arns/names nil;
// when metadata is missing for a group, policies/users default to 0.
func TestUnitIAMGroupsDataSource_flattenGroups(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		groups := []awstypes.Group{
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:group/1111"),
				CreateDate: aws.Time(createDate),
				GroupId:    aws.String("AIDA6DQOBIAD7BL4YN5OA"),
				GroupName:  aws.String("1111"),
			},
		}
		metadata := map[string]tfiam.OOSListGroupsMetadata{
			"1111": {
				Policies: 4,
				Users:    10,
			},
		}

		tfGroups, arns, groupNames := tfiam.FlattenGroups(groups, metadata)

		if got, want := len(tfGroups), 1; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if got, want := arns[0], "arn:ctyun:iam::969618309127:group/1111"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}
		if got, want := groupNames[0], "1111"; got != want {
			t.Fatalf("expected group name %q, got %q", want, got)
		}
		if got, want := tfGroups[0][names.AttrARN], "arn:ctyun:iam::969618309127:group/1111"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}
		if got, want := tfGroups[0]["create_date"], "2026-06-09T08:30:10Z"; got != want {
			t.Fatalf("expected create_date %q, got %q", want, got)
		}
		if got, want := tfGroups[0]["group_id"], "AIDA6DQOBIAD7BL4YN5OA"; got != want {
			t.Fatalf("expected group_id %q, got %q", want, got)
		}
		if got, want := tfGroups[0][names.AttrGroupName], "1111"; got != want {
			t.Fatalf("expected group_name %q, got %q", want, got)
		}
		if got, want := tfGroups[0]["policies"], 4; got != want {
			t.Fatalf("expected policies %v, got %v", want, got)
		}
		if got, want := tfGroups[0]["users"], 10; got != want {
			t.Fatalf("expected users %v, got %v", want, got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		tfGroups, arns, groupNames := tfiam.FlattenGroups(nil, nil)
		if tfGroups == nil {
			t.Fatal("expected non-nil empty groups slice")
		}
		if got, want := len(tfGroups), 0; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if arns != nil {
			t.Fatalf("expected arns nil, got %#v", arns)
		}
		if groupNames != nil {
			t.Fatalf("expected names nil, got %#v", groupNames)
		}

		tfGroups, arns, groupNames = tfiam.FlattenGroups([]awstypes.Group{}, map[string]tfiam.OOSListGroupsMetadata{})
		if tfGroups == nil {
			t.Fatal("expected non-nil empty groups slice for empty input")
		}
		if got, want := len(tfGroups), 0; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if arns != nil || groupNames != nil {
			t.Fatalf("expected arns/names nil, got arns=%#v names=%#v", arns, groupNames)
		}
	})

	t.Run("missing_metadata", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		groups := []awstypes.Group{
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:group/with-meta"),
				CreateDate: aws.Time(createDate),
				GroupId:    aws.String("GIDWITHMETA"),
				GroupName:  aws.String("with-meta"),
			},
			{
				Arn:        aws.String("arn:ctyun:iam::969618309127:group/no-meta"),
				CreateDate: aws.Time(createDate),
				GroupId:    aws.String("GIDNOMETA"),
				GroupName:  aws.String("no-meta"),
			},
		}
		metadata := map[string]tfiam.OOSListGroupsMetadata{
			"with-meta": {
				Policies: 2,
				Users:    3,
			},
			// "no-meta" intentionally absent
		}

		tfGroups, _, _ := tfiam.FlattenGroups(groups, metadata)
		if got, want := len(tfGroups), 2; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if got, want := tfGroups[0]["policies"], 2; got != want {
			t.Fatalf("expected with-meta policies %v, got %v", want, got)
		}
		if got, want := tfGroups[0]["users"], 3; got != want {
			t.Fatalf("expected with-meta users %v, got %v", want, got)
		}
		if got, want := tfGroups[1]["policies"], 0; got != want {
			t.Fatalf("expected missing-metadata policies default %v, got %v", want, got)
		}
		if got, want := tfGroups[1]["users"], 0; got != want {
			t.Fatalf("expected missing-metadata users default %v, got %v", want, got)
		}
	})
}

// TestUnitIAMGroupsDataSource_filterGroupsByNameRegex verifies local name_regex filtering:
// partial match, full match (output = input), and no match at all (empty slice).
func TestUnitIAMGroupsDataSource_filterGroupsByNameRegex(t *testing.T) {
	t.Parallel()

	groups := []awstypes.Group{
		{GroupName: aws.String("1111")},
		{GroupName: aws.String("123byrole")},
		{GroupName: aws.String("Terraform-1")},
	}

	t.Run("partial_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterGroupsByNameRegex(groups, regexp.MustCompile(`^123`))
		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		if got, want := aws.ToString(output[0].GroupName), "123byrole"; got != want {
			t.Fatalf("expected group_name %q, got %q", want, got)
		}
	})

	t.Run("all_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterGroupsByNameRegex(groups, regexp.MustCompile(`.*`))
		if got, want := len(output), len(groups); got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
		for i := range groups {
			if got, want := aws.ToString(output[i].GroupName), aws.ToString(groups[i].GroupName); got != want {
				t.Fatalf("expected group_name[%d] %q, got %q", i, want, got)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterGroupsByNameRegex(groups, regexp.MustCompile(`^nomatch-`))
		if got, want := len(output), 0; got != want {
			t.Fatalf("expected %d groups, got %d", want, got)
		}
	})
}

// TC-IAM-DGROUPS-01-Basic List (incorporating the former 03 Group Metadata Readback):
// Sub-scenario 1: create only 1 group (no bindings) without name_regex; id is the Region, and arns/names/groups match
// the full ListGroups result exactly.
// Sub-scenario 2: create multiple groups covering no binding / policy-only / user-only / both policy and user; same checks
// as sub-scenario 1, plus explicit assertions of the policies/users counts for all four variants; CheckDestroy at the end.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account; if run in parallel with
// pagination etc., the snapshot read at Apply would differ from the List count at Check time.
func TestAccIAMGroupsDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_groups.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupsDataSourceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupsDataSourceConfig_basicSingle(rName),
				Check:  testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, ""),
			},
			{
				Config: testAccGroupsDataSourceConfig_basicMultiple(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, ""),
					testAccCheckGroupsDataSourceGroupCounts(dataSourceName, rName+"-none", 0, 0),
					testAccCheckGroupsDataSourceGroupCounts(dataSourceName, rName+"-policy", 1, 0),
					testAccCheckGroupsDataSourceGroupCounts(dataSourceName, rName+"-user", 0, 1),
					testAccCheckGroupsDataSourceGroupCounts(dataSourceName, rName+"-both", 1, 1),
				),
			},
		},
	})
}

// TC-IAM-DGROUPS-02-NameRegex Filter verifies creating two groups and using name_regex to match exactly one of them:
// ExactMatch compares against "ListGroups + local filtering with the same rule"; it also explicitly asserts that the
// groups/names/arns counts are all 1 and that names contains the expected groupA; CheckDestroy at the end.
func TestAccIAMGroupsDataSource_nameRegex(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	groupA := rName + "-a"
	groupB := rName + "-b"
	dataSourceName := "data.ctyun_iam_groups.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(groupA))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupsDataSourceConfig_nameRegex(groupA, groupB),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", groupA),
				),
			},
		},
	})
}

// TC-IAM-DGROUPS-05-Empty Result verifies: first create at least 1 group (ensuring ListGroups is non-empty),
// then read with a name_regex that cannot match anything; ExactMatch compares against the filtered ListGroups result and
// explicitly asserts that groups/arns/names are all empty (due to regex filtering, not because the cloud has no groups),
// with id still being the Region; CheckDestroy at the end.
func TestAccIAMGroupsDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_groups.test"
	nameRegex := "^tf-no-such-group-zzzzzzzz$"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupsDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "0"),
				),
			},
		},
	})
}

// The ListGroups default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input + Paginator and
// cannot change MaxItems, so this case creates more data than the default page size to force the real Read through multi-page aggregation.
const testAccIAMGroupsListDefaultMaxItems = 100

// TC-IAM-DGROUPS-06-Pagination verifies that when the total number of groups in the account exceeds the ListGroups default
// MaxItems, the data source Read (empty ListGroupsInput → default pagination) still returns all groups matching name_regex.
func TestAccIAMGroupsDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	groupCount := testAccIAMGroupsListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_groups.test"
	nameRegex := fmt.Sprintf("^%s-", regexp.QuoteMeta(rName))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckGroupsPaginationQuota(ctx, t, groupCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupsDataSourceConfig_pagination(rName, groupCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupsListDefaultPageTruncated(ctx, t),
					testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "groups.#", strconv.Itoa(groupCount)),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", strconv.Itoa(groupCount)),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", strconv.Itoa(groupCount)),
				),
			},
		},
	})
}

// TC-IAM-DGROUPS-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state is
// ExactMatch-compared against the cloud ListGroups (filtered with the same name_regex); the second apply shows no diff
// (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused by unstable read-back formats of
// flattenGroups / create_date / policies / users.
// name_regex is used to bound the result set so the test can run in parallel.
func TestAccIAMGroupsDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	groupA := rName + "-a"
	groupB := rName + "-b"
	dataSourceName := "data.ctyun_iam_groups.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(groupA))

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckGroupsDataSourceExactMatchListGroups(ctx, t, dataSourceName, nameRegex),
		resource.TestCheckResourceAttr(dataSourceName, "groups.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
		resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", groupA),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupsDataSourceConfig_nameRegex(groupA, groupB),
				Check:  checks,
			},
			{
				Config: testAccGroupsDataSourceConfig_nameRegex(groupA, groupB),
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

// testAccCheckGroupsDataSourceExactMatchListGroups checks the DS state:
//   - id equals the current IAM EffectiveRealRegion;
//   - the cloud list is fetched with ListGroups (same source as the DS, including OOS metadata); if nameRegex is non-empty, it is
//     filtered locally with the same rule as the DS;
//   - arns/names/groups match the filtered result set exactly (no more, no less), with field-by-field comparison (including policies/users counts).
func testAccCheckGroupsDataSourceExactMatchListGroups(ctx context.Context, t *testing.T, dataSourceName, nameRegex string) resource.TestCheckFunc {
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
		stateCount, err := strconv.Atoi(attrs["groups.#"])
		if err != nil {
			return fmt.Errorf("parsing groups.#: %w", err)
		}

		conn := meta.IAMClient(ctx)
		cloudGroups, metadata, err := tfiam.FindGroups(ctx, conn, &iam.ListGroupsInput{})
		if err != nil {
			return fmt.Errorf("listing IAM Groups: %w", err)
		}
		if nameRegex != "" {
			r, err := regexp.Compile(nameRegex)
			if err != nil {
				return fmt.Errorf("compiling name_regex (%s): %w", nameRegex, err)
			}
			cloudGroups = tfiam.FilterGroupsByNameRegex(cloudGroups, r)
		}

		if stateCount != len(cloudGroups) {
			return fmt.Errorf("groups count mismatch: state=%d listGroups=%d", stateCount, len(cloudGroups))
		}

		cloudByARN := make(map[string]awstypes.Group, len(cloudGroups))
		cloudARNs := make(map[string]struct{}, len(cloudGroups))
		cloudNames := make(map[string]struct{}, len(cloudGroups))
		for _, group := range cloudGroups {
			arn := aws.ToString(group.Arn)
			cloudByARN[arn] = group
			cloudARNs[arn] = struct{}{}
			cloudNames[aws.ToString(group.GroupName)] = struct{}{}
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
			prefix := fmt.Sprintf("groups.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty groups.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate groups.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("groups.%d.arn %q not found in ListGroups", i, stateARN)
			}

			groupName := aws.ToString(cloud.GroupName)
			if got, want := attrs[prefix+names.AttrGroupName], groupName; got != want {
				return fmt.Errorf("bad groups.%d.group_name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"group_id"], aws.ToString(cloud.GroupId); got != want {
				return fmt.Errorf("bad groups.%d.group_id: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListGroups for %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad groups.%d.create_date: got %s, want %s", i, got, want)
			}

			groupMeta := metadata[groupName]
			if got, want := attrs[prefix+"policies"], strconv.Itoa(groupMeta.Policies); got != want {
				return fmt.Errorf("bad groups.%d.policies: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"users"], strconv.Itoa(groupMeta.Users); got != want {
				return fmt.Errorf("bad groups.%d.users: got %s, want %s", i, got, want)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("groups ARN set size mismatch after field checks: state=%d listGroups=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccCheckGroupsDataSourceGroupCounts locates an entry by group_name in the DS state and asserts the policies/users counts.
func testAccCheckGroupsDataSourceGroupCounts(dataSourceName, groupName string, wantPolicies, wantUsers int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		attrs := rs.Primary.Attributes
		count, err := strconv.Atoi(attrs["groups.#"])
		if err != nil {
			return fmt.Errorf("parsing groups.#: %w", err)
		}

		for i := 0; i < count; i++ {
			prefix := fmt.Sprintf("groups.%d.", i)
			if attrs[prefix+names.AttrGroupName] != groupName {
				continue
			}
			if got, want := attrs[prefix+"policies"], strconv.Itoa(wantPolicies); got != want {
				return fmt.Errorf("group %q policies: got %s, want %s", groupName, got, want)
			}
			if got, want := attrs[prefix+"users"], strconv.Itoa(wantUsers); got != want {
				return fmt.Errorf("group %q users: got %s, want %s", groupName, got, want)
			}
			return nil
		}

		return fmt.Errorf("group %q not found in %s groups list", groupName, dataSourceName)
	}
}

func testAccCheckGroupsDataSourceDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckUserGroupMembershipDestroy(ctx, t),
		testAccCheckGroupPolicyAttachmentDestroy(ctx, t),
		testAccCheckUserDestroy(ctx, t),
		testAccCheckGroupDestroy(ctx, t),
		testAccCheckPolicyDestroy(ctx, t),
	)
}

func testAccGroupsDataSourceConfig_basicSingle(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

data "ctyun_iam_groups" "test" {
  depends_on = [ctyun_iam_group.test]
}
`, rName)
}

func testAccGroupsDataSourceConfig_basicMultiple(rName string) string {
	return fmt.Sprintf(`
# Keep the ctyun_iam_group.test from step 1 to avoid a race between destroying the old group and the DS Read in the same apply.
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

resource "ctyun_iam_group" "none" {
  name = "%[1]s-none"
}

resource "ctyun_iam_group" "policy" {
  name = "%[1]s-policy"
}

resource "ctyun_iam_group" "user" {
  name = "%[1]s-user"
}

resource "ctyun_iam_group" "both" {
  name = "%[1]s-both"
}

resource "ctyun_iam_policy" "policy" {
  name        = "%[1]s-policy"
  description = "groups ds basic"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Action": ["iam:ChangePassword"],
    "Effect": "Allow",
    "Resource": "*"
  }]
}
EOF
}

resource "ctyun_iam_policy" "both" {
  name        = "%[1]s-both-policy"
  description = "groups ds basic both"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Action": ["iam:ChangePassword"],
    "Effect": "Allow",
    "Resource": "*"
  }]
}
EOF
}

resource "ctyun_iam_group_policy_attachment" "policy" {
  group      = ctyun_iam_group.policy.name
  policy_arn = ctyun_iam_policy.policy.arn
}

resource "ctyun_iam_group_policy_attachment" "both" {
  group      = ctyun_iam_group.both.name
  policy_arn = ctyun_iam_policy.both.arn
}

resource "ctyun_iam_user" "user" {
  name          = "%[1]s-user"
  force_destroy = true
}

resource "ctyun_iam_user" "both" {
  name          = "%[1]s-both-user"
  force_destroy = true
}

resource "ctyun_iam_user_group_membership" "user" {
  user  = ctyun_iam_user.user.name
  group = ctyun_iam_group.user.name
}

resource "ctyun_iam_user_group_membership" "both" {
  user  = ctyun_iam_user.both.name
  group = ctyun_iam_group.both.name
}

data "ctyun_iam_groups" "test" {
  depends_on = [
    ctyun_iam_group.test,
    ctyun_iam_group.none,
    ctyun_iam_group_policy_attachment.policy,
    ctyun_iam_group_policy_attachment.both,
    ctyun_iam_user_group_membership.user,
    ctyun_iam_user_group_membership.both,
  ]
}
`, rName)
}

func testAccGroupsDataSourceConfig_nameRegex(groupA, groupB string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "a" {
  name = %[1]q
}

resource "ctyun_iam_group" "b" {
  name = %[2]q
}

data "ctyun_iam_groups" "test" {
  name_regex = format("^%%s$", %[1]q)

  depends_on = [
    ctyun_iam_group.a,
    ctyun_iam_group.b,
  ]
}
`, groupA, groupB)
}

func testAccGroupsDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  name = %[1]q
}

data "ctyun_iam_groups" "test" {
  name_regex = "^tf-no-such-group-zzzzzzzz$"

  depends_on = [ctyun_iam_group.test]
}
`, rName)
}

// testAccPreCheckGroupsPaginationQuota confirms that the remaining GroupsQuota is sufficient to create pageSize+1 groups.
func testAccPreCheckGroupsPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroupsQuota)]
	if !ok {
		t.Fatalf("GroupsQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeGroups)]
	if !ok {
		t.Fatalf("Groups missing in SummaryMap: %#v", out.SummaryMap)
	}
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient GroupsQuota remaining=%d need=%d (quota=%d used=%d)", remaining, need, quota, used)
	}
}

// testAccCheckGroupsListDefaultPageTruncated fetches one page (without paging further) using the same empty ListGroupsInput as
// the DS Read, confirming the account has more than the default MaxItems with IsTruncated=true (otherwise this case has not
// actually exercised the DS pagination path).
func testAccCheckGroupsListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListGroups(ctx, &iam.ListGroupsInput{})
		if err != nil {
			return fmt.Errorf("ListGroups(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListGroups page (MaxItems default %d); first page returned %d groups — DS Read would not exercise pagination", testAccIAMGroupsListDefaultMaxItems, len(page.Groups))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

func testAccGroupsDataSourceConfig_pagination(rName string, groupCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_group" "test" {
  count = %[2]d
  name  = "%[1]s-${count.index}"
}

data "ctyun_iam_groups" "test" {
  name_regex = format("^%%s-", %[1]q)

  depends_on = [ctyun_iam_group.test]
}
`, rName, groupCount)
}
