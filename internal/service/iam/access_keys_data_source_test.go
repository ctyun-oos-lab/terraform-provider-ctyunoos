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
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMAccessKeysDataSource_schema verifies the ctyun_iam_access_keys data source schema:
// it includes access_keys/user; access_keys is a TypeSet with access_key_id/create_date/status fields; user has a ValidateFunc.
func TestUnitIAMAccessKeysDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceAccessKeys()

	for _, name := range []string{"access_keys", "user"} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	if dataSource.Schema["access_keys"].Type != schema.TypeSet {
		t.Fatalf("expected access_keys TypeSet, got %v", dataSource.Schema["access_keys"].Type)
	}

	keySchema := dataSource.Schema["access_keys"].Elem.(*schema.Resource).Schema
	for _, name := range []string{"access_key_id", "create_date", names.AttrStatus} {
		if _, ok := keySchema[name]; !ok {
			t.Fatalf("expected access_keys.%s schema", name)
		}
	}

	if dataSource.Schema["user"].ValidateFunc == nil {
		t.Fatal("expected user ValidateFunc")
	}
}

// TestUnitIAMAccessKeysDataSource_nameValidation verifies ValidateFunc for the user argument
// (covering the former TC-IAM-DAK-03 User Validation Matrix):
// valid special characters and the length limit of 64 pass; empty values, 65 characters, and invalid characters such as spaces, !, &, and * fail at the schema stage.
func TestUnitIAMAccessKeysDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	validateFunc := tfiam.DataSourceAccessKeys().Schema["user"].ValidateFunc
	if validateFunc == nil {
		t.Fatal("expected user ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_special_chars", value: "User_-,.+@=123", errContains: nil},
		{name: "valid_special_chars_alt", value: "test-1+2_.@hello", errContains: nil},
		{name: "valid_max_length", value: strings.Repeat("a", 64), errContains: nil},
		{name: "empty", value: "", errContains: []string{
			"expected length of user to be in the range (1 - 64)",
		}},
		{name: "too_long", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of user to be in the range (1 - 64)",
		}},
		{name: "invalid_space", value: "t est", errContains: []string{
			"invalid value for user",
			"must contain only alphanumeric characters and _-,.+@=",
		}},
		{name: "invalid_exclamation", value: "invalid user!", errContains: []string{
			"invalid value for user",
			"must contain only alphanumeric characters and _-,.+@=",
		}},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", errContains: []string{
			"invalid value for user",
			"must contain only alphanumeric characters and _-,.+@=",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validateFunc(tc.value, "user")
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for user=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for user=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for user missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TestUnitIAMAccessKeysDataSource_flattenAccessKeys verifies flattenAccessKey:
// the normal path maps access_key_id/create_date/status; nil AccessKeyId/CreateDate and empty Status do not write the corresponding key;
// FlattenAccessKeys(nil) and an empty slice return nil.
func TestUnitIAMAccessKeysDataSource_flattenAccessKeys(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		createDate := time.Date(2026, 6, 9, 8, 30, 10, 0, time.UTC)
		output := tfiam.FlattenAccessKeys([]awstypes.AccessKeyMetadata{
			{
				AccessKeyId: aws.String("AKIAEXAMPLEKEYID001"),
				CreateDate:  aws.Time(createDate),
				Status:      awstypes.StatusTypeActive,
			},
		})

		if got, want := len(output), 1; got != want {
			t.Fatalf("expected %d access keys, got %d", want, got)
		}

		m, ok := output[0].(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", output[0])
		}
		if got, want := m["access_key_id"], "AKIAEXAMPLEKEYID001"; got != want {
			t.Fatalf("access_key_id = %v, want %v", got, want)
		}
		if got, want := m["create_date"], "2026-06-09T08:30:10Z"; got != want {
			t.Fatalf("create_date = %v, want %v", got, want)
		}
		if got, want := fmt.Sprint(m[names.AttrStatus]), string(awstypes.StatusTypeActive); got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	})

	t.Run("nil_fields", func(t *testing.T) {
		t.Parallel()

		// Set UserName so the structure is non-zero; otherwise flattenAccessKeys skips the zero-value element.
		m := tfiam.FlattenAccessKey(awstypes.AccessKeyMetadata{
			UserName:    aws.String("example-user"),
			AccessKeyId: nil,
			CreateDate:  nil,
			Status:      "",
		})
		if m == nil {
			t.Fatal("expected non-nil map for non-zero AccessKeyMetadata")
		}
		if _, ok := m["access_key_id"]; ok {
			t.Fatalf("expected access_key_id omitted, got %#v", m["access_key_id"])
		}
		if _, ok := m["create_date"]; ok {
			t.Fatalf("expected create_date omitted, got %#v", m["create_date"])
		}
		if _, ok := m[names.AttrStatus]; ok {
			t.Fatalf("expected status omitted, got %#v", m[names.AttrStatus])
		}
		if got, want := len(m), 0; got != want {
			t.Fatalf("expected empty map, got %#v", m)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		if got := tfiam.FlattenAccessKeys(nil); got != nil {
			t.Fatalf("expected nil for nil input, got %#v", got)
		}
		if got := tfiam.FlattenAccessKeys([]awstypes.AccessKeyMetadata{}); got != nil {
			t.Fatalf("expected nil for empty input, got %#v", got)
		}
	})
}

// TC-IAM-DAK-01-Basic Single Key
// Verifies reading by user after creating a user and one access key: DS state matches remote ListAccessKeys
// (without comparing managed resource state); CheckDestroy runs at completion.
func TestAccIAMAccessKeysDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_access_keys.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckAccessKeyDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeysDataSourceConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "access_keys.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "user", rName),
				),
			},
		},
	})
}

// TC-IAM-DAK-02-Multiple Keys
// Verifies reading after creating two access keys for one user: access_keys exactly matches the remote ListAccessKeys set; CheckDestroy runs at completion.
func TestAccIAMAccessKeysDataSource_twoKeys(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_access_keys.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckAccessKeyDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeysDataSourceConfig_twoKeys(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "access_keys.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "user", rName),
				),
			},
		},
	})
}

// TC-IAM-DAK-04-Empty Result
// Verifies reading a user who exists but has no access keys: access_keys is empty, id/user is the user name,
// and the result matches remote ListAccessKeys; CheckDestroy runs at completion.
func TestAccIAMAccessKeysDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_access_keys.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeysDataSourceConfig_empty(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "access_keys.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "user", rName),
				),
			},
		},
	})
}

// TC-IAM-DAK-05-Missing User Negative verifies that reading a nonexistent user returns an explicit NotFound-class error
// and leaves no data.ctyun_iam_access_keys state behind (no partial result).
func TestAccIAMAccessKeysDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAccessKeysDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccAccessKeysDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckAccessKeysDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DAK-05-Other Account User Negative verifies that reading a cross-account user fails and leaves no partial DS state.
// CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND are required; this ACC creates the user through the second-account SDK and destroys it in t.Cleanup.
func TestAccIAMAccessKeysDataSource_otherAccount(t *testing.T) {
	crossAccountUser := testAccCrossAccountUserName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAccessKeysDataSourceConfig_otherAccount(crossAccountUser),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccAccessKeysDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckAccessKeysDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DAK-06-Read Stability verifies that repeated applies with the same configuration both match remote ListAccessKeys
// and that the second apply has no diff (TypeSet order is irrelevant, so changed return order does not cause a persistent plan).
func TestAccIAMAccessKeysDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_access_keys.test"

	checks := resource.ComposeTestCheckFunc(
		testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, "access_keys.#", "2"),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttr(dataSourceName, "user", rName),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckAccessKeyDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeysDataSourceConfig_twoKeys(rName),
				Check:  checks,
			},
			{
				Config: testAccAccessKeysDataSourceConfig_twoKeys(rName),
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

// ListAccessKeys has a default MaxItems of 100, consistent with AWS IAM and OOS; DS Read uses empty Input plus a Paginator
// and cannot change MaxItems, so this case creates more keys than the default page size to force real Read through multi-page aggregation.
// The OOS default AccessKeysPerUserQuota is 150, enough to cover pageSize+1.
const testAccIAMListAccessKeysDefaultMaxItems = 100

// TC-IAM-DAK-07-Pagination verifies that when a user's access-key count exceeds ListAccessKeys default MaxItems,
// data source Read (empty ListAccessKeysInput -> default pagination) still aggregates and returns every access_keys value, matching the cloud result.
func TestAccIAMAccessKeysDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	keyCount := testAccIAMListAccessKeysDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_access_keys.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckAccessKeysPerUserQuota(ctx, t, keyCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckAccessKeyDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeysDataSourceConfig_pagination(rName, keyCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAccessKeysListDefaultPageTruncated(ctx, t, dataSourceName),
					testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "access_keys.#", strconv.Itoa(keyCount)),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "user", rName),
				),
			},
		},
	})
}

// testAccCheckAccessKeysDataSourceStateMatchesCloud verifies DS state against remote ListAccessKeys
// without relying on managed resource state: it compares id/user, the access_keys TypeSet, and each field.
// （access_key_id/create_date/status）
func testAccCheckAccessKeysDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("bad state id: empty")
		}

		userName := rs.Primary.Attributes["user"]
		if userName == "" {
			return fmt.Errorf("bad state user: empty")
		}
		if rs.Primary.ID != userName {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, userName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		cloudKeys, err := tfiam.FindAccessKeysByUser(ctx, conn, userName)
		if err != nil {
			return fmt.Errorf("listing access keys for user (%s): %w", userName, err)
		}

		stateKeys, err := collectAccessKeysTypeSet(rs.Primary.Attributes)
		if err != nil {
			return err
		}
		if len(stateKeys) != len(cloudKeys) {
			return fmt.Errorf("access_keys count mismatch: state=%d listAccessKeys=%d", len(stateKeys), len(cloudKeys))
		}

		cloudByID := make(map[string]awstypes.AccessKeyMetadata, len(cloudKeys))
		for _, key := range cloudKeys {
			cloudByID[aws.ToString(key.AccessKeyId)] = key
		}

		for accessKeyID, state := range stateKeys {
			cloud, ok := cloudByID[accessKeyID]
			if !ok {
				return fmt.Errorf("access_key_id %q not found in ListAccessKeys", accessKeyID)
			}
			if cloud.CreateDate == nil {
				return fmt.Errorf("CreateDate is nil in ListAccessKeys for access_key_id %s", accessKeyID)
			}
			if got, want := state["create_date"], aws.ToTime(cloud.CreateDate).Format(time.RFC3339); got != want {
				return fmt.Errorf("bad access_keys[%s].create_date: got %s, want %s", accessKeyID, got, want)
			}
			if got, want := state[names.AttrStatus], string(cloud.Status); got != want {
				return fmt.Errorf("bad access_keys[%s].status: got %s, want %s", accessKeyID, got, want)
			}
		}

		if len(stateKeys) != len(cloudByID) {
			return fmt.Errorf("access_keys id set size mismatch after field checks: state=%d listAccessKeys=%d", len(stateKeys), len(cloudByID))
		}

		return nil
	}
}

func collectAccessKeysTypeSet(attrs map[string]string) (map[string]map[string]string, error) {
	countStr, ok := attrs["access_keys.#"]
	if !ok {
		return nil, fmt.Errorf("access_keys.# not set")
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return nil, fmt.Errorf("parsing access_keys.#: %w", err)
	}

	out := make(map[string]map[string]string)
	for k, v := range attrs {
		const suffix = ".access_key_id"
		if !strings.HasPrefix(k, "access_keys.") || !strings.HasSuffix(k, suffix) {
			continue
		}
		hash := strings.TrimSuffix(strings.TrimPrefix(k, "access_keys."), suffix)
		if hash == "" || hash == "#" {
			continue
		}
		prefix := "access_keys." + hash + "."
		out[v] = map[string]string{
			"access_key_id":  v,
			"create_date":    attrs[prefix+"create_date"],
			names.AttrStatus: attrs[prefix+names.AttrStatus],
		}
	}

	if len(out) != count {
		return nil, fmt.Errorf("access_keys TypeSet parse size mismatch: parsed=%d count=%d", len(out), count)
	}
	return out, nil
}

func testAccCheckAccessKeysDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_access_keys.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q user=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes["user"],
			)
		}
		return nil
	}
}

func testAccAccessKeysDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_access_key" "test" {
  user = ctyun_iam_user.test.name
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_access_key.test.user
}
`, rName)
}

func testAccAccessKeysDataSourceConfig_twoKeys(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_access_key" "test_1" {
  user = ctyun_iam_user.test.name
}

resource "ctyun_iam_access_key" "test_2" {
  user = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_access_key.test_1]
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_access_key.test_1.user

  depends_on = [ctyun_iam_access_key.test_2]
}
`, rName)
}

func testAccAccessKeysDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_user.test.name
}
`, rName)
}

func testAccAccessKeysDataSourceConfig_notFound(rName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_access_keys" "test" {
  user = %[1]q
}
`, rName)
}

func testAccAccessKeysDataSourceConfig_otherAccount(userName string) string {
	return fmt.Sprintf(`
# The cross-account user is created with second-account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test completes.
data "ctyun_iam_access_keys" "test" {
  user = %[1]q
}
`, userName)
}

func testAccAccessKeysDataSourceConfig_pagination(rName string, keyCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_access_key" "test" {
  count = %[2]d
  user  = ctyun_iam_user.test.name
}

data "ctyun_iam_access_keys" "test" {
  user = ctyun_iam_user.test.name

  depends_on = [ctyun_iam_access_key.test]
}
`, rName, keyCount)
}

func testAccAccessKeysDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed access_keys data source read left no residual state
`
}

// testAccPreCheckAccessKeysPerUserQuota confirms that AccessKeysPerUserQuota allows one user to create the requested number of access keys.
func testAccPreCheckAccessKeysPerUserQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	key := string(awstypes.SummaryKeyTypeAccessKeysPerUserQuota)
	quota, ok := out.SummaryMap[key]
	if !ok || quota <= 0 {
		t.Fatalf("AccessKeysPerUserQuota missing or invalid in SummaryMap: %#v", out.SummaryMap)
	}
	if int(quota) < need {
		t.Skipf("insufficient AccessKeysPerUserQuota=%d need=%d", quota, need)
	}
}

// testAccCheckAccessKeysListDefaultPageTruncated uses the same empty ListAccessKeysInput as DS Read
// to fetch one page without pagination, confirming that the user has more access keys than default MaxItems and IsTruncated=true
// (otherwise this case does not actually exercise the DS pagination path).
func testAccCheckAccessKeysListDefaultPageTruncated(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}
		userName := rs.Primary.Attributes["user"]
		if userName == "" {
			return fmt.Errorf("bad user: empty")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
			UserName: aws.String(userName),
		})
		if err != nil {
			return fmt.Errorf("ListAccessKeys(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListAccessKeys page (MaxItems default %d); first page returned %d keys — DS Read would not exercise pagination", testAccIAMListAccessKeysDefaultMaxItems, len(page.AccessKeyMetadata))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}
