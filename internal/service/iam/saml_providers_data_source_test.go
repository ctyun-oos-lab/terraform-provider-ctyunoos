// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
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

// TestUnitIAMSAMLProvidersDataSource_schema verifies the schema shape of ctyun_iam_saml_providers:
// it contains arns/names/name_regex/providers; it does not contain path/path_prefix/tags/tags_all;
// the providers elements contain arn/create_date/name/saml_metadata_document/saml_provider_uuid/valid_until;
// name_regex has a ValidateFunc.
func TestUnitIAMSAMLProvidersDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceSAMLProviders()

	for _, name := range []string{names.AttrARNs, names.AttrNames, "name_regex", "providers"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s in schema", name)
		}
	}

	for _, name := range []string{"path", "path_prefix", "tags", "tags_all"} {
		if _, ok := dataSource.Schema[name]; ok {
			t.Fatalf("did not expect %s in schema", name)
		}
	}

	providerSchema := dataSource.Schema["providers"].Elem.(*schema.Resource).Schema
	for _, name := range []string{
		names.AttrARN,
		"create_date",
		names.AttrName,
		"saml_metadata_document",
		"saml_provider_uuid",
		"valid_until",
	} {
		if _, ok := providerSchema[name]; !ok {
			t.Fatalf("expected providers.%s schema", name)
		}
	}

	if dataSource.Schema["name_regex"].ValidateFunc == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}
}

// TestUnitIAMSAMLProvidersDataSource_nameRegexValidation verifies the schema ValidateFunc of name_regex:
// valid regular expressions pass; various invalid regex forms return clear parsing errors.
// Covers the former TC-IAM-DSAMLS-04 / TestAccIAMSAMLProvidersDataSource_invalidRegex (invalid "[" etc. fail at the schema stage; no separate ACC anymore).
func TestUnitIAMSAMLProvidersDataSource_nameRegexValidation(t *testing.T) {
	t.Parallel()

	validate := tfiam.DataSourceSAMLProviders().Schema["name_regex"].ValidateFunc
	if validate == nil {
		t.Fatal("expected name_regex ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_prefix", value: "^tf-", errContains: nil},
		{name: "valid_exact", value: `^my-saml-123$`, errContains: nil},
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

// TestUnitIAMSAMLProvidersDataSource_filterByNameRegex verifies local name_regex filtering:
// partial match, full match (output = input), no match at all (nil empty slice), empty input, and skipping an entry when its ARN fails to parse.
func TestUnitIAMSAMLProvidersDataSource_filterByNameRegex(t *testing.T) {
	t.Parallel()

	providers := []awstypes.SAMLProviderListEntry{
		{Arn: aws.String("arn:ctyun:iam::abc123def456g:saml-provider/1111")},
		{Arn: aws.String("arn:ctyun:iam::abc123def456g:saml-provider/Terraform-1")},
		{Arn: aws.String("arn:ctyun:iam::abc123def456g:saml-provider/other")},
	}

	t.Run("partial_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterSAMLProvidersByNameRegex(providers, regexp.MustCompile(`^Terraform`))
		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d providers, got %d", want, got)
		}
		if got, want := aws.ToString(output[0].Arn), "arn:ctyun:iam::abc123def456g:saml-provider/Terraform-1"; got != want {
			t.Fatalf("expected arn %q, got %q", want, got)
		}
	})

	t.Run("all_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterSAMLProvidersByNameRegex(providers, regexp.MustCompile(`.*`))
		if got, want := len(output), len(providers); got != want {
			t.Fatalf("expected %d providers, got %d", want, got)
		}
		for i := range providers {
			if got, want := aws.ToString(output[i].Arn), aws.ToString(providers[i].Arn); got != want {
				t.Fatalf("expected arn[%d] %q, got %q", i, want, got)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterSAMLProvidersByNameRegex(providers, regexp.MustCompile(`^nomatch-`))
		if output != nil {
			t.Fatalf("expected nil slice for no match, got %#v", output)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()

		output := tfiam.FilterSAMLProvidersByNameRegex(nil, regexp.MustCompile(`.*`))
		if output != nil {
			t.Fatalf("expected nil slice for nil input, got %#v", output)
		}
		output = tfiam.FilterSAMLProvidersByNameRegex([]awstypes.SAMLProviderListEntry{}, regexp.MustCompile(`.*`))
		if output != nil {
			t.Fatalf("expected nil slice for empty input, got %#v", output)
		}
	})

	t.Run("invalid_arn_skipped", func(t *testing.T) {
		t.Parallel()

		mixed := []awstypes.SAMLProviderListEntry{
			{Arn: aws.String("not-an-arn")},
			{Arn: aws.String("arn:ctyun:iam::abc123def456g:saml-provider/1111")},
			{Arn: aws.String("")},
			{Arn: aws.String("arn:incomplete")},
			{Arn: aws.String("arn:ctyun:iam::abc123def456g:saml-provider/Terraform-1")},
		}

		output := tfiam.FilterSAMLProvidersByNameRegex(mixed, regexp.MustCompile(`.*`))
		if got, want := len(output), 2; got != want {
			t.Fatalf("expected %d providers after skipping invalid ARNs, got %d (%#v)", want, got, output)
		}
		if got, want := aws.ToString(output[0].Arn), "arn:ctyun:iam::abc123def456g:saml-provider/1111"; got != want {
			t.Fatalf("expected arn[0] %q, got %q", want, got)
		}
		if got, want := aws.ToString(output[1].Arn), "arn:ctyun:iam::abc123def456g:saml-provider/Terraform-1"; got != want {
			t.Fatalf("expected arn[1] %q, got %q", want, got)
		}
	})
}

// TestUnitIAMSAMLProvidersDataSource_flattenSAMLProviderListEntry verifies flattenSAMLProviderListEntry:
// an ARN parse failure returns an error; the normal path maps name/arn/create_date plus the GetSAMLProvider details;
// valid_until prefers detail.ValidUntil and falls back to listEntry.ValidUntil; nil times become empty strings.
func TestUnitIAMSAMLProvidersDataSource_flattenSAMLProviderListEntry(t *testing.T) {
	t.Parallel()

	providerARN := "arn:ctyun:iam::abc123def456g:saml-provider/example"
	createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
	listValidUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	detailValidUntil := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	t.Run("arn_parse_error", func(t *testing.T) {
		t.Parallel()

		_, err := tfiam.FlattenSAMLProviderListEntry(context.Background(), nil, awstypes.SAMLProviderListEntry{
			Arn: aws.String("not-an-arn"),
		})
		if err == nil {
			t.Fatal("expected error for invalid ARN")
		}
		if !strings.Contains(err.Error(), "parsing IAM SAML Provider ARN") {
			t.Fatalf("expected ARN parse error, got: %s", err)
		}
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		conn := testIAMClientGetSAMLProvider(t, testGetSAMLProviderResponseXML("meta-doc", "uuid-1", &detailValidUntil), http.StatusOK)
		tfMap, err := tfiam.FlattenSAMLProviderListEntry(context.Background(), conn, awstypes.SAMLProviderListEntry{
			Arn:        aws.String(providerARN),
			CreateDate: aws.Time(createDate),
			ValidUntil: aws.Time(listValidUntil),
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		want := map[string]any{
			names.AttrARN:            providerARN,
			"create_date":            "2026-06-09T08:30:10Z",
			names.AttrName:           "example",
			"saml_metadata_document": "meta-doc",
			"saml_provider_uuid":     "uuid-1",
			"valid_until":            "2026-12-31T23:59:59Z",
		}
		for k, wantVal := range want {
			if gotVal := tfMap[k]; gotVal != wantVal {
				t.Fatalf("expected %s %v, got %v", k, wantVal, gotVal)
			}
		}
	})

	t.Run("valid_until_fallback", func(t *testing.T) {
		t.Parallel()

		conn := testIAMClientGetSAMLProvider(t, testGetSAMLProviderResponseXML("meta-doc", "uuid-1", nil), http.StatusOK)
		tfMap, err := tfiam.FlattenSAMLProviderListEntry(context.Background(), conn, awstypes.SAMLProviderListEntry{
			Arn:        aws.String(providerARN),
			CreateDate: aws.Time(createDate),
			ValidUntil: aws.Time(listValidUntil),
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got, want := tfMap["valid_until"], "2026-07-01T00:00:00Z"; got != want {
			t.Fatalf("expected valid_until fallback %q, got %q", want, got)
		}
	})

	t.Run("nil_times", func(t *testing.T) {
		t.Parallel()

		conn := testIAMClientGetSAMLProvider(t, testGetSAMLProviderResponseXML("meta-doc", "uuid-1", nil), http.StatusOK)
		tfMap, err := tfiam.FlattenSAMLProviderListEntry(context.Background(), conn, awstypes.SAMLProviderListEntry{
			Arn:        aws.String(providerARN),
			CreateDate: nil,
			ValidUntil: nil,
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got, want := tfMap["create_date"], ""; got != want {
			t.Fatalf("expected empty create_date, got %q", got)
		}
		if got, want := tfMap["valid_until"], ""; got != want {
			t.Fatalf("expected empty valid_until, got %q", got)
		}
	})
}

// TC-IAM-DSAMLS-01-Basic List (incorporating the former 03 Detail Expansion field checks):
// Sub-scenario 1: create only 1 SAML provider without name_regex; id is the Region, and arns/names/providers
// match the full ListSAMLProviders result exactly (no more, no less), with each entry's detail fields matching List + GetSAMLProvider.
// Sub-scenario 2: create multiple providers; same checks as sub-scenario 1. CheckDestroy at the end.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account; if run in parallel with
// pagination etc., the snapshot read at Apply would differ from the List count at Check time because of fixtures created by other tests (typical: state=1 vs list=N).
func TestAccIAMSAMLProvidersDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_providers.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProvidersDataSourceConfig_basicSingle(rName, idpEntityID),
				Check:  testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, ""),
			},
			{
				Config: testAccSAMLProvidersDataSourceConfig_basicMultiple(rName, idpEntityID),
				Check:  testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, ""),
			},
		},
	})
}

// TC-IAM-DSAMLS-02-NameRegex Filter verifies creating two SAML providers and using name_regex to match exactly one of them:
// ExactMatch compares against "full ListSAMLProviders + local filtering with the same rule"; it also explicitly asserts that
// the providers/names/arns counts are all 1 and that names contains the expected providerA; CheckDestroy at the end.
func TestAccIAMSAMLProvidersDataSource_nameRegex(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	providerA := rName + "-a"
	providerB := rName + "-b"
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_providers.test"
	nameRegex := fmt.Sprintf("^%s$", regexp.QuoteMeta(providerA))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProvidersDataSourceConfig_nameRegex(providerA, providerB, idpEntityID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "providers.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "1"),
					resource.TestCheckTypeSetElemAttr(dataSourceName, "names.*", providerA),
				),
			},
		},
	})
}

// TC-IAM-DSAMLS-05-Empty Result verifies: first create at least 1 provider (ensuring ListSAMLProviders is non-empty),
// then read with a name_regex that cannot match anything; ExactMatch compares against the filtered list and explicitly asserts
// that providers/arns/names are all empty (due to regex filtering, not because the cloud has no providers), with id still
// being the Region; CheckDestroy at the end.
func TestAccIAMSAMLProvidersDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_providers.test"
	nameRegex := "^tf-no-such-saml-zzzzzzzz$"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProvidersDataSourceConfig_empty(rName, idpEntityID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "providers.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", "0"),
				),
			},
		},
	})
}

// OOS ListSAMLProviders: MaxItems defaults to 100; beyond that IsTruncated=true and a Marker is required to continue.
// The DS Read cannot change MaxItems, so this case creates more data than the default page size to force the real Read through multi-page aggregation.
const testAccIAMSAMLProvidersListDefaultMaxItems = 100

// TC-IAM-DSAMLS-06-Pagination verifies that when the total number of SAML providers in the account exceeds the
// ListSAMLProviders default MaxItems, the data source Read (default pagination aggregation) still returns all providers matching name_regex.
func TestAccIAMSAMLProvidersDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	providerCount := testAccIAMSAMLProvidersListDefaultMaxItems + 1
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_providers.test"
	nameRegex := fmt.Sprintf("^%s-", regexp.QuoteMeta(rName))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckSAMLProvidersPaginationQuota(ctx, t, providerCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProvidersDataSourceConfig_pagination(rName, idpEntityID, providerCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSAMLProvidersListDefaultPageTruncated(ctx, t),
					testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, nameRegex),
					resource.TestCheckResourceAttr(dataSourceName, "providers.#", strconv.Itoa(providerCount)),
					resource.TestCheckResourceAttr(dataSourceName, "names.#", strconv.Itoa(providerCount)),
					resource.TestCheckResourceAttr(dataSourceName, "arns.#", strconv.Itoa(providerCount)),
				),
			},
		},
	})
}

// TC-IAM-DSAMLS-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state is
// ExactMatch-compared against the full cloud ListSAMLProviders result (including create_date / valid_until); the second apply
// shows no diff (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused by unstable read-back formats of RFC3339
// time fields. The configuration reuses basicMultiple.
// Reason for serial execution: without name_regex, ExactMatch compares against the whole account.
func TestAccIAMSAMLProvidersDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	idpEntityID := fmt.Sprintf("https://%s", acctest.RandomDomainName())
	dataSourceName := "data.ctyun_iam_saml_providers.test"

	checks := testAccCheckSAMLProvidersDataSourceExactMatchList(ctx, t, dataSourceName, "")

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckSAMLProviderDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccSAMLProvidersDataSourceConfig_basicMultiple(rName, idpEntityID),
				Check:  checks,
			},
			{
				Config: testAccSAMLProvidersDataSourceConfig_basicMultiple(rName, idpEntityID),
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

// testAccCheckSAMLProvidersDataSourceExactMatchList checks that the DS state matches the full paginated cloud
// ListSAMLProviders set exactly (optionally filtered locally with the same name_regex as the DS), and calls GetSAMLProvider
// for each entry to compare create_date/name/saml_metadata_document/saml_provider_uuid/valid_until; it does not compare
// against managed resource state.
func testAccCheckSAMLProvidersDataSourceExactMatchList(ctx context.Context, t *testing.T, dataSourceName, nameRegex string) resource.TestCheckFunc {
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
		stateCount, err := strconv.Atoi(attrs["providers.#"])
		if err != nil {
			return fmt.Errorf("parsing providers.#: %w", err)
		}

		conn := meta.IAMClient(ctx)
		cloudProviders, err := tfiam.FindSAMLProviders(ctx, conn)
		if err != nil {
			return fmt.Errorf("listing IAM SAML Providers: %w", err)
		}
		if nameRegex != "" {
			r, err := regexp.Compile(nameRegex)
			if err != nil {
				return fmt.Errorf("compiling name_regex (%s): %w", nameRegex, err)
			}
			cloudProviders = tfiam.FilterSAMLProvidersByNameRegex(cloudProviders, r)
		}

		if stateCount != len(cloudProviders) {
			return fmt.Errorf("providers count mismatch: state=%d listSAMLProviders=%d", stateCount, len(cloudProviders))
		}

		cloudByARN := make(map[string]awstypes.SAMLProviderListEntry, len(cloudProviders))
		cloudARNs := make(map[string]struct{}, len(cloudProviders))
		cloudNames := make(map[string]struct{}, len(cloudProviders))
		for _, provider := range cloudProviders {
			arn := aws.ToString(provider.Arn)
			cloudByARN[arn] = provider
			cloudARNs[arn] = struct{}{}
			name, err := tfiam.NameFromSAMLProviderARN(arn)
			if err != nil {
				return fmt.Errorf("parsing ListSAMLProviders ARN (%s): %w", arn, err)
			}
			cloudNames[name] = struct{}{}
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
			prefix := fmt.Sprintf("providers.%d.", i)
			stateARN := attrs[prefix+names.AttrARN]
			if stateARN == "" {
				return fmt.Errorf("empty providers.%d.arn", i)
			}
			if _, dup := seenARN[stateARN]; dup {
				return fmt.Errorf("duplicate providers.%d.arn %q in state", i, stateARN)
			}
			seenARN[stateARN] = struct{}{}

			listEntry, ok := cloudByARN[stateARN]
			if !ok {
				return fmt.Errorf("providers.%d.arn %q not found in ListSAMLProviders", i, stateARN)
			}

			wantName, err := tfiam.NameFromSAMLProviderARN(stateARN)
			if err != nil {
				return err
			}
			if got := attrs[prefix+names.AttrName]; got != wantName {
				return fmt.Errorf("bad providers.%d.name: got %s, want %s", i, got, wantName)
			}

			if listEntry.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListSAMLProviders for %s", stateARN)
			}
			if got, want := attrs[prefix+"create_date"], aws.ToTime(listEntry.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad providers.%d.create_date: got %s, want %s", i, got, want)
			}

			detail, err := tfiam.FindSAMLProviderByARN(ctx, conn, stateARN)
			if err != nil {
				return fmt.Errorf("reading IAM SAML Provider (%s): %w", stateARN, err)
			}
			if got, want := attrs[prefix+"saml_metadata_document"], aws.ToString(detail.SAMLMetadataDocument); got != want {
				return fmt.Errorf("bad providers.%d.saml_metadata_document: state %q != cloud %q", i, got, want)
			}
			if got, want := attrs[prefix+"saml_provider_uuid"], aws.ToString(detail.SAMLProviderUUID); got != want {
				return fmt.Errorf("bad providers.%d.saml_provider_uuid: got %s, want %s", i, got, want)
			}

			wantValidUntil := ""
			if detail.ValidUntil != nil {
				wantValidUntil = aws.ToTime(detail.ValidUntil).Format(time.RFC3339)
			} else if listEntry.ValidUntil != nil {
				wantValidUntil = aws.ToTime(listEntry.ValidUntil).Format(time.RFC3339)
			}
			if got := attrs[prefix+"valid_until"]; got != wantValidUntil {
				return fmt.Errorf("bad providers.%d.valid_until: got %s, want %s", i, got, wantValidUntil)
			}
		}

		if len(seenARN) != len(cloudByARN) {
			return fmt.Errorf("providers ARN set size mismatch after field checks: state=%d listSAMLProviders=%d", len(seenARN), len(cloudByARN))
		}

		return nil
	}
}

// testAccPreCheckSAMLProvidersPaginationQuota confirms that the remaining ProvidersQuota is sufficient to create pageSize+1 providers.
// The AWS SDK SummaryKeyType only has Providers, not ProvidersQuota; the quota key in the OOS SummaryMap is exactly "ProvidersQuota".
func testAccPreCheckSAMLProvidersPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap["ProvidersQuota"]
	if !ok {
		t.Fatalf("ProvidersQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeProviders)]
	if !ok {
		t.Fatalf("Providers missing in SummaryMap: %#v", out.SummaryMap)
	}
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient ProvidersQuota remaining=%d need=%d (quota=%d used=%d)", remaining, need, quota, used)
	}
}

// testAccCheckSAMLProvidersListDefaultPageTruncated uses a single-page ListSAMLProviders call from the same source as the DS
// (without paging further), confirming the account has more than the default MaxItems with IsTruncated=true (otherwise this
// case has not actually exercised the DS pagination path).
func testAccCheckSAMLProvidersListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, marker, truncated, err := tfiam.ListSAMLProvidersPage(ctx, conn, "")
		if err != nil {
			return fmt.Errorf("ListSAMLProviders(default MaxItems) single page: %w", err)
		}
		if !truncated {
			return fmt.Errorf("expected IsTruncated=true for default ListSAMLProviders page (MaxItems default %d); first page returned %d providers — DS Read would not exercise pagination", testAccIAMSAMLProvidersListDefaultMaxItems, len(page))
		}
		if marker == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

func testAccSAMLProvidersDataSourceConfig_basicSingle(rName, idpEntityID string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

data "ctyun_iam_saml_providers" "test" {
  depends_on = [ctyun_iam_saml_provider.test]
}
`, rName, idpEntityID)
}

func testAccSAMLProvidersDataSourceConfig_basicMultiple(rName, idpEntityID string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "a" {
  name                   = "%[1]s-a"
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

resource "ctyun_iam_saml_provider" "b" {
  name                   = "%[1]s-b"
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

resource "ctyun_iam_saml_provider" "c" {
  name                   = "%[1]s-c"
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

data "ctyun_iam_saml_providers" "test" {
  depends_on = [
    ctyun_iam_saml_provider.a,
    ctyun_iam_saml_provider.b,
    ctyun_iam_saml_provider.c,
  ]
}
`, rName, idpEntityID)
}

func testAccSAMLProvidersDataSourceConfig_nameRegex(providerA, providerB, idpEntityID string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "a" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[3]q })
}

resource "ctyun_iam_saml_provider" "b" {
  name                   = %[2]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[3]q })
}

data "ctyun_iam_saml_providers" "test" {
  name_regex = format("^%%s$", %[1]q)

  depends_on = [
    ctyun_iam_saml_provider.a,
    ctyun_iam_saml_provider.b,
  ]
}
`, providerA, providerB, idpEntityID)
}

func testAccSAMLProvidersDataSourceConfig_empty(rName, idpEntityID string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  name                   = %[1]q
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

data "ctyun_iam_saml_providers" "test" {
  name_regex = "^tf-no-such-saml-zzzzzzzz$"

  depends_on = [ctyun_iam_saml_provider.test]
}
`, rName, idpEntityID)
}

func testAccSAMLProvidersDataSourceConfig_pagination(rName, idpEntityID string, providerCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_saml_provider" "test" {
  count                  = %[3]d
  name                   = "%[1]s-${count.index}"
  saml_metadata_document = templatefile("./test-fixtures/saml-metadata.xml.tpl", { entity_id = %[2]q })
}

data "ctyun_iam_saml_providers" "test" {
  name_regex = format("^%%s-", %[1]q)

  depends_on = [ctyun_iam_saml_provider.test]
}
`, rName, idpEntityID, providerCount)
}

func testGetSAMLProviderResponseXML(metadata, uuid string, validUntil *time.Time) string {
	validUntilXML := ""
	if validUntil != nil {
		validUntilXML = "<ValidUntil>" + validUntil.UTC().Format(time.RFC3339) + "</ValidUntil>"
	}

	return `<GetSAMLProviderResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetSAMLProviderResult>
    <SAMLMetadataDocument>` + metadata + `</SAMLMetadataDocument>
    <SAMLProviderUUID>` + uuid + `</SAMLProviderUUID>` + validUntilXML + `
  </GetSAMLProviderResult>
  <ResponseMetadata>
    <RequestId>test-request-id</RequestId>
  </ResponseMetadata>
</GetSAMLProviderResponse>`
}

func testIAMClientGetSAMLProvider(t *testing.T, responseXML string, status int) *iam.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(responseXML))
	}))
	t.Cleanup(srv.Close)

	return iam.NewFromConfig(aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
		BaseEndpoint:     aws.String(srv.URL),
		HTTPClient:       srv.Client(),
		RetryMaxAttempts: 1,
		Retryer:          func() aws.Retryer { return aws.NopRetryer{} },
	})
}
