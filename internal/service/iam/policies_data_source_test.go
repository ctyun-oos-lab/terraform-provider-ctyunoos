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

// TestUnitIAMPoliciesDataSource_schema verifies the schema shape of ctyun_iam_policies:
// it contains arns/names/name_regex/policies; it does not contain upstream only_attached/path_prefix etc.;
// the policies elements contain the documented attributes without is_attachable/update_date.
func TestUnitIAMPoliciesDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourcePolicies()

	for _, name := range []string{names.AttrARNs, names.AttrNames, "name_regex", "policies"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	for _, name := range []string{"only_attached", "policy_name", "policy_ids", "path_prefix"} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("unexpected %s schema", name)
		}
	}

	policySchema := dataSource.Schema["policies"].Elem.(*schema.Resource).Schema
	for _, name := range []string{
		names.AttrARN,
		"attachment_count",
		"create_date",
		names.AttrDescription,
		names.AttrName,
		"policy_id",
		names.AttrScope,
	} {
		if _, ok := policySchema[name]; !ok {
			t.Fatalf("expected policies.%s schema", name)
		}
	}

	for _, name := range []string{"is_attachable", "update_date", "policy_name"} {
		if _, ok := policySchema[name]; ok {
			t.Fatalf("unexpected policies.%s schema", name)
		}
	}

	if dataSource.Schema["name_regex"].ValidateFunc == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}
}

// TestUnitIAMPoliciesDataSource_nameRegexValidation verifies the schema ValidateFunc of name_regex:
// valid regular expressions pass; various invalid regex forms return clear parsing errors (covers the former TC-IAM-DPOLICIES-05).
//
// Positive cases: errContains is empty; negative cases: every substring in errContains must appear in the error text.
func TestUnitIAMPoliciesDataSource_nameRegexValidation(t *testing.T) {
	t.Parallel()

	validate := tfiam.DataSourcePolicies().Schema["name_regex"].ValidateFunc
	if validate == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_prefix", value: "^tf-", errContains: nil},
		{name: "valid_exact", value: `^my-policy-123$`, errContains: nil},
		{name: "valid_empty", value: "", errContains: nil},

		{name: "missing_closing_bracket", value: "[", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing closing ]",
		}},
		{name: "incomplete_character_class", value: "[a-", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing closing ]",
		}},
		{name: "missing_closing_paren", value: "(", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing closing )",
		}},
		{name: "incomplete_non_capturing_group", value: "(?:", errContains: []string{
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
		{name: "repetition_plus_no_argument", value: "+", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing argument to repetition operator",
		}},
		{name: "repetition_brace_no_argument", value: "{2}", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"missing argument to repetition operator",
		}},
		{name: "invalid_repeat_count", value: "a{2,1}", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"invalid repeat count",
		}},
		{name: "invalid_named_capture_empty", value: "(?P<>a)", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"invalid named capture",
		}},
		{name: "invalid_named_capture_unclosed", value: "(?P<name", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"invalid named capture",
		}},
		{name: "unsupported_perl_syntax", value: "(?z)", errContains: []string{
			`"name_regex"`,
			"error parsing regexp",
			"invalid or unsupported Perl syntax",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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

// TestUnitIAMPoliciesDataSource_flattenPolicies verifies the flattenPolicies normal path:
// mapping of arn/attachment_count/create_date/description/name/policy_id/scope, plus the arns/names slices.
func TestUnitIAMPoliciesDataSource_flattenPolicies(t *testing.T) {
	t.Parallel()

	createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
	policies := []awstypes.Policy{
		{
			Arn:             aws.String("arn:ctyun:iam::969618309127:policy/11111333"),
			AttachmentCount: aws.Int32(3),
			CreateDate:      aws.Time(createDate),
			Description:     aws.String("<script>alert(1);</script>"),
			PolicyId:        aws.String("ANPA6DQOBIAD5Q6PELP7"),
			PolicyName:      aws.String("123"),
		},
	}
	scopes := map[string]string{
		"ANPA6DQOBIAD5Q6PELP7": "Local",
	}

	tfPolicies, arns, policyNames := tfiam.FlattenPolicies(policies, scopes)

	if got, want := len(tfPolicies), 1; got != want {
		t.Fatalf("expected %d policies, got %d", want, got)
	}

	if got, want := arns[0], "arn:ctyun:iam::969618309127:policy/11111333"; got != want {
		t.Fatalf("expected arn %q, got %q", want, got)
	}

	if got, want := policyNames[0], "123"; got != want {
		t.Fatalf("expected policy name %q, got %q", want, got)
	}

	if got, want := tfPolicies[0][names.AttrARN], "arn:ctyun:iam::969618309127:policy/11111333"; got != want {
		t.Fatalf("expected arn %q, got %q", want, got)
	}

	if got, want := tfPolicies[0]["attachment_count"], 3; got != want {
		t.Fatalf("expected attachment_count %v, got %v", want, got)
	}

	if got, want := tfPolicies[0]["create_date"], "2026-06-09T08:30:10Z"; got != want {
		t.Fatalf("expected create_date %q, got %q", want, got)
	}

	if got, want := tfPolicies[0][names.AttrDescription], "<script>alert(1);</script>"; got != want {
		t.Fatalf("expected description %q, got %q", want, got)
	}

	if got, want := tfPolicies[0][names.AttrName], "123"; got != want {
		t.Fatalf("expected name %q, got %q", want, got)
	}

	if got, want := tfPolicies[0]["policy_id"], "ANPA6DQOBIAD5Q6PELP7"; got != want {
		t.Fatalf("expected policy_id %q, got %q", want, got)
	}

	if got, want := tfPolicies[0][names.AttrScope], "Local"; got != want {
		t.Fatalf("expected scope %q, got %q", want, got)
	}
}

// TestUnitIAMPoliciesDataSource_flattenPolicies_empty verifies empty input: a non-nil empty policies slice with arns/names nil.
func TestUnitIAMPoliciesDataSource_flattenPolicies_empty(t *testing.T) {
	t.Parallel()

	tfPolicies, arns, policyNames := tfiam.FlattenPolicies(nil, nil)
	if tfPolicies == nil {
		t.Fatal("expected non-nil empty policies slice")
	}
	if got, want := len(tfPolicies), 0; got != want {
		t.Fatalf("expected %d policies, got %d", want, got)
	}
	if arns != nil {
		t.Fatalf("expected arns nil, got %#v", arns)
	}
	if policyNames != nil {
		t.Fatalf("expected names nil, got %#v", policyNames)
	}

	tfPolicies, arns, policyNames = tfiam.FlattenPolicies([]awstypes.Policy{}, map[string]string{})
	if tfPolicies == nil {
		t.Fatal("expected non-nil empty policies slice for empty input")
	}
	if got, want := len(tfPolicies), 0; got != want {
		t.Fatalf("expected %d policies, got %d", want, got)
	}
	if arns != nil || policyNames != nil {
		t.Fatalf("expected arns/names nil, got arns=%#v names=%#v", arns, policyNames)
	}
}

// TestUnitIAMPoliciesDataSource_flattenPolicies_missingScope verifies that scope defaults to "" when scopes is missing for a policy.
func TestUnitIAMPoliciesDataSource_flattenPolicies_missingScope(t *testing.T) {
	t.Parallel()

	createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
	policies := []awstypes.Policy{
		{
			Arn:             aws.String("arn:ctyun:iam::969618309127:policy/with-scope"),
			AttachmentCount: aws.Int32(1),
			CreateDate:      aws.Time(createDate),
			PolicyId:        aws.String("ANPAWITHSCOPE"),
			PolicyName:      aws.String("with-scope"),
		},
		{
			Arn:             aws.String("arn:ctyun:iam::969618309127:policy/no-scope"),
			AttachmentCount: aws.Int32(0),
			CreateDate:      aws.Time(createDate),
			PolicyId:        aws.String("ANPANOSCOPE"),
			PolicyName:      aws.String("no-scope"),
		},
	}
	scopes := map[string]string{
		"ANPAWITHSCOPE": "Local",
	}

	tfPolicies, _, _ := tfiam.FlattenPolicies(policies, scopes)
	if got, want := len(tfPolicies), 2; got != want {
		t.Fatalf("expected %d policies, got %d", want, got)
	}
	if got, want := tfPolicies[0][names.AttrScope], "Local"; got != want {
		t.Fatalf("expected with-scope scope %q, got %q", want, got)
	}
	if got, want := tfPolicies[1][names.AttrScope], ""; got != want {
		t.Fatalf("expected missing-scope default %q, got %q", want, got)
	}
}

// TestUnitIAMPoliciesDataSource_filterPoliciesByNameRegex verifies local name_regex filtering:
// partial match, full match (output = input), and no match at all (nil empty slice).
func TestUnitIAMPoliciesDataSource_filterPoliciesByNameRegex(t *testing.T) {
	t.Parallel()

	policies := []awstypes.Policy{
		{PolicyName: aws.String("11111333")},
		{PolicyName: aws.String("1111233")},
		{PolicyName: aws.String("123e")},
	}

	t.Run("partial_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterPoliciesByNameRegex(policies, regexp.MustCompile(`^123`))
		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		if got, want := aws.ToString(output[0].PolicyName), "123e"; got != want {
			t.Fatalf("expected policy name %q, got %q", want, got)
		}
	})

	t.Run("all_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterPoliciesByNameRegex(policies, regexp.MustCompile(`.*`))
		if got, want := len(output), len(policies); got != want {
			t.Fatalf("expected %d policies, got %d", want, got)
		}
		for i := range policies {
			if got, want := aws.ToString(output[i].PolicyName), aws.ToString(policies[i].PolicyName); got != want {
				t.Fatalf("expected policy_name[%d] %q, got %q", i, want, got)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterPoliciesByNameRegex(policies, regexp.MustCompile(`^nomatch-`))
		if output != nil {
			t.Fatalf("expected nil slice for no match, got %#v", output)
		}
	})
}

// TC-IAM-DPOLICIES-01-Basic List (incorporating the former 03 Metadata field checks):
// Sub-scenario 1: create only 1 policy without name_regex; id is the Region, and arns/names/policies match the full
// ListPolicies result exactly (no more, no less), with each policy's fields matching ListPolicies.
// Sub-scenario 2: create multiple policies; same checks as sub-scenario 1. CheckDestroy at the end.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account; if run in parallel with
// pagination etc., the snapshot read at Apply would differ from the List count at Check time because of fixtures created by other tests.
func TestAccIAMPoliciesDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policies.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			// Sub-scenario 1: a single policy
			{
				Config: testAccPoliciesDataSourceConfig_basicSingle(rName),
				Check:  testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, ""),
			},
			// Sub-scenario 2: multiple policies
			{
				Config: testAccPoliciesDataSourceConfig_basicMultiple(rName),
				Check:  testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, ""),
			},
		},
	})
}

// TC-IAM-DPOLICIES-02-NameRegex Filter verifies that name_regex returns only policies with matching names;
// ExactMatch compares against "ListPolicies + local filtering with the same rule"; it also explicitly asserts that the
// policies/names/arns counts are all 1 and that names contains the expected policyA; CheckDestroy at the end.
func TestAccIAMPoliciesDataSource_nameRegex(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyA := rName + "-a"
	policyB := rName + "-b"
	dataSourceName := "data.ctyun_iam_policies.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(policyA))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPoliciesDataSourceConfig_nameRegex(policyA, policyB),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", policyA),
				),
			},
		},
	})
}

// TC-IAM-DPOLICIES-04-Empty Result verifies: first create at least 1 policy (ensuring ListPolicies is non-empty),
// then read with a name_regex that cannot match anything; ExactMatch compares against the filtered ListPolicies result and
// explicitly asserts that policies/arns/names are all empty (due to regex filtering, not because the cloud has no policies),
// with id still being the Region.
func TestAccIAMPoliciesDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policies.test"
	// A fixed regex unrelated to the fixture policy names, ensuring the filtered result is empty.
	nameRegex := "^tf-no-such-policy-zzzzzzzz$"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPoliciesDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "0"),
				),
			},
		},
	})
}

// ListPolicies documentation: MaxItems defaults to 100; beyond that IsTruncated=true and a Marker is required to continue.
// The DS Read cannot change MaxItems, so this case creates more data than the default page size to force the real Read through multi-page aggregation.
const testAccIAMPoliciesListDefaultMaxItems = 100

// TC-IAM-DPOLICIES-06-Pagination verifies that when the total number of policies in the account exceeds the ListPolicies
// default MaxItems, the data source Read (empty ListPoliciesInput → default pagination) still returns all policies matching
// name_regex.
// Creating 101 policies at once consumes the account-level PoliciesQuota; a serial Test is used to avoid contending for
// PoliciesPerAccount with other pagination/Limit ACCs.
func TestAccIAMPoliciesDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	policyCount := testAccIAMPoliciesListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_policies.test"
	nameRegex := fmt.Sprintf("^%s-", regexp.QuoteMeta(rName))

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckPoliciesPaginationQuota(ctx, t, policyCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPoliciesDataSourceConfig_pagination(rName, policyCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckPoliciesListDefaultPageTruncated(ctx, t),
					testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", strconv.Itoa(policyCount)),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", strconv.Itoa(policyCount)),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", strconv.Itoa(policyCount)),
				),
			},
		},
	})
}

// TC-IAM-DPOLICIES-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state is
// ExactMatch-compared against the cloud ListPolicies (filtered with the same name_regex); the second apply shows no diff
// (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused by unstable read-back formats of flattenPolicies /
// create_date. name_regex is used to bound the result set so the test can run in parallel.
func TestAccIAMPoliciesDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	policyA := rName + "-a"
	policyB := rName + "-b"
	dataSourceName := "data.ctyun_iam_policies.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(policyA))

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx, t, dataSourceName, nameRegex),
		resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
		resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", policyA),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccPoliciesDataSourceConfig_nameRegex(policyA, policyB),
				Check:  checks,
			},
			{
				Config: testAccPoliciesDataSourceConfig_nameRegex(policyA, policyB),
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

// testAccCheckPoliciesDataSourceExactMatchListPolicies checks the DS state:
//   - id equals the current IAM EffectiveRealRegion (requirement: region name);
//   - the cloud list is fetched with ListPolicies (same source as the DS); if nameRegex is non-empty, it is filtered locally
//     with the same rule as the DS;
//   - arns/names/policies match the filtered result set exactly (no more, no less), with field-by-field comparison.
func testAccCheckPoliciesDataSourceExactMatchListPolicies(ctx context.Context, t *testing.T, dataSourceName, nameRegex string) resource.TestCheckFunc {
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
		stateCount, err := strconv.Atoi(attrs["policies.#"])
		if err != nil {
			return fmt.Errorf("parsing policies.#: %w", err)
		}

		conn := meta.IAMClient(ctx)
		cloudPolicies, scopes, err := tfiam.FindPolicies(ctx, conn, &iam.ListPoliciesInput{}, tfiam.WithOOSIAMListPoliciesResponse(""))
		if err != nil {
			return fmt.Errorf("listing IAM Policies: %w", err)
		}
		if nameRegex != "" {
			r, err := regexp.Compile(nameRegex)
			if err != nil {
				return fmt.Errorf("compiling name_regex (%s): %w", nameRegex, err)
			}
			cloudPolicies = tfiam.FilterPoliciesByNameRegex(cloudPolicies, r)
		}

		if stateCount != len(cloudPolicies) {
			return fmt.Errorf("policies count mismatch: state=%d listPolicies=%d", stateCount, len(cloudPolicies))
		}

		cloudByARN := make(map[string]awstypes.Policy, len(cloudPolicies))
		cloudARNs := make(map[string]struct{}, len(cloudPolicies))
		cloudNames := make(map[string]struct{}, len(cloudPolicies))
		for _, p := range cloudPolicies {
			arn := aws.ToString(p.Arn)
			cloudByARN[arn] = p
			cloudARNs[arn] = struct{}{}
			cloudNames[aws.ToString(p.PolicyName)] = struct{}{}
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
			prefix := fmt.Sprintf("policies.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty policies.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate policies.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			cloud, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("policies.%d.arn %q not found in ListPolicies", i, stateARN)
			}

			if got, want := attrs[prefix+names.AttrName], aws.ToString(cloud.PolicyName); got != want {
				return fmt.Errorf("bad policies.%d.name: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+names.AttrDescription], aws.ToString(cloud.Description); got != want {
				return fmt.Errorf("bad policies.%d.description: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"policy_id"], aws.ToString(cloud.PolicyId); got != want {
				return fmt.Errorf("bad policies.%d.policy_id: got %s, want %s", i, got, want)
			}
			if got, want := attrs[prefix+"attachment_count"], strconv.Itoa(int(aws.ToInt32(cloud.AttachmentCount))); got != want {
				return fmt.Errorf("bad policies.%d.attachment_count: got %s, want %s", i, got, want)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListPolicies for %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad policies.%d.create_date: got %s, want %s", i, got, want)
			}
			scope := scopes[aws.ToString(cloud.PolicyId)]
			if scope == "" {
				return fmt.Errorf("ListPolicies response did not include scope for policy_id %s", aws.ToString(cloud.PolicyId))
			}
			if got := attrs[prefix+names.AttrScope]; got != scope {
				return fmt.Errorf("bad policies.%d.scope: got %s, want %s", i, got, scope)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("policies ARN set size mismatch after field checks: state=%d listPolicies=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccPreCheckPoliciesPaginationQuota confirms that the remaining PoliciesQuota is sufficient to create need more policies.
// Shared by the pagination ACC and other cases like attachmentLimitNegative that create many policies at once.
func testAccPreCheckPoliciesPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypePoliciesQuota)]
	if !ok {
		t.Fatalf("PoliciesQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used, ok := out.SummaryMap[string(awstypes.SummaryKeyTypePolicies)]
	if !ok {
		t.Fatalf("Policies missing in SummaryMap: %#v", out.SummaryMap)
	}
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient PoliciesQuota remaining=%d need=%d (quota=%d used=%d)", remaining, need, quota, used)
	}
}

// testAccCheckPoliciesListDefaultPageTruncated fetches one page (without paging further) using the same empty ListPoliciesInput
// as the DS Read, confirming the account has more than the default MaxItems with IsTruncated=true (otherwise this case has not
// actually exercised the DS pagination path).
func testAccCheckPoliciesListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListPolicies(ctx, &iam.ListPoliciesInput{}, tfiam.WithOOSIAMListPoliciesResponse(""))
		if err != nil {
			return fmt.Errorf("ListPolicies(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListPolicies page (MaxItems default %d); first page returned %d policies — DS Read would not exercise pagination", testAccIAMPoliciesListDefaultMaxItems, len(page.Policies))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

func collectTypeSetValues(attrs map[string]string, setAttr string) map[string]struct{} {
	out := map[string]struct{}{}
	n, err := strconv.Atoi(attrs[setAttr+".#"])
	if err != nil || n == 0 {
		return out
	}
	prefix := setAttr + "."
	for k, v := range attrs {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix || k == setAttr+".#" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func setExactMatch(label string, state, cloud map[string]struct{}) error {
	if len(state) != len(cloud) {
		return fmt.Errorf("%s count mismatch: state=%d cloud=%d", label, len(state), len(cloud))
	}
	for v := range state {
		if _, ok := cloud[v]; !ok {
			return fmt.Errorf("%s has extra value %q not in cloud", label, v)
		}
	}
	for v := range cloud {
		if _, ok := state[v]; !ok {
			return fmt.Errorf("%s missing cloud value %q", label, v)
		}
	}
	return nil
}

func testAccPoliciesDataSourceConfig_policyDocument() string {
	return `
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
`
}

func testAccPoliciesDataSourceConfig_basicSingle(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "test"
%[2]s
}

data "ctyun_iam_policies" "test" {
  depends_on = [ctyun_iam_policy.test]
}
`, rName, testAccPoliciesDataSourceConfig_policyDocument())
}

func testAccPoliciesDataSourceConfig_basicMultiple(rName string) string {
	return fmt.Sprintf(`
# Keep the ctyun_iam_policy.test from step 1 to avoid a race between destroying the old policy and the DS Read in the same apply.
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "test"
%[2]s
}

resource "ctyun_iam_policy" "a" {
  name        = "%[1]s-a"
  description = "test-a"
%[2]s
}

resource "ctyun_iam_policy" "b" {
  name        = "%[1]s-b"
  description = "test-b"
%[2]s
}

resource "ctyun_iam_policy" "c" {
  name        = "%[1]s-c"
  description = "test-c"
%[2]s
}

data "ctyun_iam_policies" "test" {
  depends_on = [
    ctyun_iam_policy.test,
    ctyun_iam_policy.a,
    ctyun_iam_policy.b,
    ctyun_iam_policy.c,
  ]
}
`, rName, testAccPoliciesDataSourceConfig_policyDocument())
}

func testAccPoliciesDataSourceConfig_nameRegex(policyA, policyB string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "a" {
  name        = %[1]q
  description = "test"
%[3]s
}

resource "ctyun_iam_policy" "b" {
  name        = %[2]q
  description = "test"
%[3]s
}

data "ctyun_iam_policies" "test" {
  name_regex = format("^%%s$", %[1]q)

  depends_on = [
    ctyun_iam_policy.a,
    ctyun_iam_policy.b,
  ]
}
`, policyA, policyB, testAccPoliciesDataSourceConfig_policyDocument())
}

func testAccPoliciesDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  name        = %[1]q
  description = "test"
%[2]s
}

data "ctyun_iam_policies" "test" {
  name_regex = "^tf-no-such-policy-zzzzzzzz$"

  depends_on = [ctyun_iam_policy.test]
}
`, rName, testAccPoliciesDataSourceConfig_policyDocument())
}

func testAccPoliciesDataSourceConfig_pagination(rName string, policyCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_policy" "test" {
  count       = %[2]d
  name        = "%[1]s-${count.index}"
  description = "pagination-${count.index}"
%[3]s
}

data "ctyun_iam_policies" "test" {
  name_regex = format("^%%s-", %[1]q)

  depends_on = [ctyun_iam_policy.test]
}
`, rName, policyCount, testAccPoliciesDataSourceConfig_policyDocument())
}
