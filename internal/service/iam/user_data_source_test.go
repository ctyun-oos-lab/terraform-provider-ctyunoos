// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMUserDataSource_schema verifies the schema shape of the ctyun_iam_user data source:
// it contains arn/ip_last_used/password_last_used/user_name/user_id/tags; it does not contain create_date;
// user_name is Required and has a ValidateFunc.
func TestUnitIAMUserDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceUser()

	for _, name := range []string{
		names.AttrARN,
		"ip_last_used",
		"password_last_used",
		names.AttrUserName,
		"user_id",
		names.AttrTags,
	} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	if _, ok := dataSource.Schema["create_date"]; ok {
		t.Fatalf("unexpected create_date schema")
	}

	userNameSchema := dataSource.Schema[names.AttrUserName]
	if !userNameSchema.Required || userNameSchema.Optional {
		t.Fatalf("expected user_name to be Required (not Optional), got Required=%v Optional=%v", userNameSchema.Required, userNameSchema.Optional)
	}
	if userNameSchema.ValidateFunc == nil {
		t.Fatal("expected user_name ValidateFunc")
	}
}

// TestUnitIAMUserDataSource_nameValidation verifies the ValidateFunc of the ctyun_iam_user data source input user_name
// (covers the former TC-IAM-DUSER-04 Name Validation Matrix):
// valid special characters and the length bounds of 1 (lower) and 64 (upper) pass; empty values, 65 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMUserDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	validateFunc := tfiam.DataSourceUser().Schema[names.AttrUserName].ValidateFunc
	if validateFunc == nil {
		t.Fatal("expected user_name ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_special_chars", value: "User_-,.+@=123", errContains: nil},
		{name: "valid_special_chars_alt", value: "test-1+2_.@hello", errContains: nil},
		{name: "valid_min_length", value: "a", errContains: nil},
		{name: "valid_max_length", value: strings.Repeat("a", 64), errContains: nil},
		{name: "empty", value: "", errContains: []string{
			"expected length of user_name to be in the range (1 - 64)",
		}},
		{name: "too_long", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of user_name to be in the range (1 - 64)",
		}},
		{name: "invalid_space", value: "t est", errContains: []string{
			"invalid value for user_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "invalid_exclamation", value: "invalid user!", errContains: []string{
			"invalid value for user_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", errContains: []string{
			"invalid value for user_name",
			"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validateFunc(tc.value, names.AttrUserName)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for user_name=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for user_name=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for user_name missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TC-IAM-DUSER-01-Basic Lookup / TC-IAM-DUSER-03-IPLastUsed Readback
// verifies reading by user_name after creating a user with tags: the DS state matches the cloud GetUser result (including
// tags; not compared against managed resource state); ip_last_used/password_last_used are always asserted to exist, and are
// compared when the cloud returns concrete values; CheckDestroy at the end.
func TestAccIAMUserDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_user.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig_basic(userName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckUserDataSourceAttrPresent(dataSourceName, "ip_last_used"),
					testAccCheckUserDataSourceAttrPresent(dataSourceName, "password_last_used"),
				),
			},
		},
	})
}

// TC-IAM-DUSER-05-Missing User Negative verifies that reading a non-existent user_name returns a clear NotFound-style error,
// and that no data.ctyun_iam_user remains in state (no half-written result).
func TestAccIAMUserDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				// After a failed read the state should remain empty; if a DS residue were left, the empty-config plan would be non-empty (Destroy).
				Config:             testAccUserDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DUSER-05-Other Account Negative verifies that reading a user that exists under another account cannot be found by
// name under the current account, expecting the same behavior as NotFound (NoSuchEntity / 404, etc.), with no
// data.ctyun_iam_user left in state.
// Requires CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND; this ACC creates the user with the second account via SDK and destroys it with t.Cleanup at the end.
func TestAccIAMUserDataSource_otherAccount(t *testing.T) {
	crossAccountUser := testAccCrossAccountUserName(t)
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserDataSourceConfig_otherAccountUser(crossAccountUser),
				ExpectError: regexache.MustCompile(`(?s)(couldn't find\s+resource|NoSuchEntity|StatusCode: 404)`),
			},
			{
				Config:             testAccUserDataSourceConfig_emptyState(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				Check:              testAccCheckUserDataSourceAbsentFromState(),
			},
		},
	})
}

// TC-IAM-DUSER-06-LastLogin Populated verifies that when reading a user that already has last-login information,
// ip_last_used / password_last_used both have concrete non-empty values matching the cloud GetUser result.
// Requires OOS_IAM_USER_LAST_LOGIN (user name only); the test is Skipped before ParallelTest if unset. The user is not destroyed.
func TestAccIAMUserDataSource_lastLoginPopulated(t *testing.T) {
	acctest.SkipIfNotRunningAcceptanceTests(t)
	userName := os.Getenv("OOS_IAM_USER_LAST_LOGIN")
	if userName == "" {
		t.Skip("skipping: OOS_IAM_USER_LAST_LOGIN must be configured")
	}
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_user.test"

	acctest.PreCheck(ctx, t)
	testAccPreCheckUserLastLogin(ctx, t, userName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig_lastLogin(userName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, userName),
					testAccCheckUserDataSourceAttrNonEmpty(dataSourceName, "ip_last_used"),
					testAccCheckUserDataSourceAttrNonEmpty(dataSourceName, "password_last_used"),
				),
			},
		},
	})
}

// TC-IAM-DUSER-07-Read Stability verifies two consecutive applies of the same configuration: at each step the DS state matches
// the cloud GetUser result; the second apply shows no diff (PreApply / PostApplyPostRefresh), avoiding perpetual plans caused
// by unstable read-back formats of password_last_used / ip_last_used. The configuration reuses basic. CheckDestroy at the end.
func TestAccIAMUserDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_user.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	checks := resource.ComposeTestCheckFunc(
		testAccCheckUserDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		testAccCheckUserDataSourceAttrPresent(dataSourceName, "ip_last_used"),
		testAccCheckUserDataSourceAttrPresent(dataSourceName, "password_last_used"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig_basic(userName),
				Check:  checks,
			},
			{
				Config: testAccUserDataSourceConfig_basic(userName),
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

// testAccPreCheckUserLastLogin confirms that the user specified by the environment variable exists and its last-login fields are non-empty; otherwise the test is Skipped.
func testAccPreCheckUserLastLogin(ctx context.Context, t *testing.T, userName string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	output, err := conn.GetUser(ctx, &iam.GetUserInput{
		UserName: aws.String(userName),
	}, tfiam.WithOOSIAMGetUserResponse())
	if err != nil {
		t.Skipf("last-login user %q not available: %s", userName, err)
	}
	if output == nil || output.User == nil {
		t.Skipf("last-login user %q empty GetUser result", userName)
	}
	if output.User.PasswordLastUsed == nil {
		t.Skipf("last-login user %q missing PasswordLastUsed", userName)
	}
	ipLastUsed, _ := output.ResultMetadata.Get(tfiam.OOSIAMUserIPLastUsedMetadataKey).(string)
	if ipLastUsed == "" {
		t.Skipf("last-login user %q missing IPLastUsed", userName)
	}
}

// testAccCheckUserDataSourceStateMatchesCloud checks that the data source state matches the cloud GetUser result without
// relying on managed resource state: the cloud user is fetched by the DS user_name (including the IPLastUsed middleware),
// comparing id/user_id/user_name/arn/tags (tags are the same as the DS, taken from the Tags returned by GetUser);
// password_last_used / ip_last_used are compared against the cloud (when empty on the cloud, state should be "").
func testAccCheckUserDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
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

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := conn.GetUser(ctx, &iam.GetUserInput{
			UserName: aws.String(userName),
		}, tfiam.WithOOSIAMGetUserResponse())
		if err != nil {
			return fmt.Errorf("reading IAM User (%s): %w", userName, err)
		}
		if output == nil || output.User == nil {
			return fmt.Errorf("IAM User (%s) not found in cloud", userName)
		}

		user := output.User
		wantUserID := aws.ToString(user.UserId)
		wantUserName := aws.ToString(user.UserName)

		if rs.Primary.ID != wantUserID {
			return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, wantUserID)
		}
		if got := rs.Primary.Attributes["user_id"]; got != wantUserID {
			return fmt.Errorf("bad user_id: got %s, want %s", got, wantUserID)
		}
		if got := rs.Primary.Attributes[names.AttrUserName]; got != wantUserName {
			return fmt.Errorf("bad user_name: got %s, want %s", got, wantUserName)
		}
		if got, want := rs.Primary.Attributes[names.AttrARN], aws.ToString(user.Arn); got != want {
			return fmt.Errorf("bad arn: got %s, want %s", got, want)
		}

		// Same as the DS setTagsOut: uses iam.keyValueTags to convert []awstypes.Tag.
		// tftags.New(user.Tags) must not be used: New does not recognize that type and would fall through to default, yielding an empty map.
		cloudTags := tfiam.KeyValueTags(ctx, user.Tags).Map()
		stateTags := collectTypeMapAttributes(rs.Primary.Attributes, names.AttrTags)
		if err := mapExactMatch(names.AttrTags, stateTags, cloudTags); err != nil {
			return err
		}

		if user.PasswordLastUsed != nil {
			want := aws.ToTime(user.PasswordLastUsed).Format(time.RFC3339)
			if got := rs.Primary.Attributes["password_last_used"]; got != want {
				return fmt.Errorf("bad password_last_used: got %s, want %s", got, want)
			}
		} else if got := rs.Primary.Attributes["password_last_used"]; got != "" {
			return fmt.Errorf("bad password_last_used: got %q, want empty (cloud PasswordLastUsed is nil)", got)
		}

		if ipLastUsed, ok := output.ResultMetadata.Get(tfiam.OOSIAMUserIPLastUsedMetadataKey).(string); ok && ipLastUsed != "" {
			if got := rs.Primary.Attributes["ip_last_used"]; got != ipLastUsed {
				return fmt.Errorf("bad ip_last_used: got %s, want %s", got, ipLastUsed)
			}
		} else if got := rs.Primary.Attributes["ip_last_used"]; got != "" {
			return fmt.Errorf("bad ip_last_used: got %q, want empty (cloud IPLastUsed not returned)", got)
		}

		return nil
	}
}

// testAccCheckUserDataSourceAttrPresent asserts that the specified attribute key exists in the data source state (the value may be an empty string).
func testAccCheckUserDataSourceAttrPresent(dataSourceName, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		if _, ok := rs.Primary.Attributes[key]; !ok {
			return fmt.Errorf("%s: attribute %q not present", dataSourceName, key)
		}

		return nil
	}
}

// testAccCheckUserDataSourceAttrNonEmpty asserts that the specified attribute exists in the data source state and is a non-empty string.
func testAccCheckUserDataSourceAttrNonEmpty(dataSourceName, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		got, ok := rs.Primary.Attributes[key]
		if !ok {
			return fmt.Errorf("%s: attribute %q not present", dataSourceName, key)
		}
		if got == "" {
			return fmt.Errorf("%s: attribute %q is empty, want non-empty", dataSourceName, key)
		}

		return nil
	}
}

func collectTypeMapAttributes(attrs map[string]string, mapAttr string) map[string]string {
	out := make(map[string]string)
	prefix := mapAttr + "."
	for k, v := range attrs {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		key := strings.TrimPrefix(k, prefix)
		if key == "%" {
			continue
		}
		out[key] = v
	}
	return out
}

func mapExactMatch(attr string, state, cloud map[string]string) error {
	if len(state) != len(cloud) {
		return fmt.Errorf("%s count mismatch: state=%d cloud=%d", attr, len(state), len(cloud))
	}
	for k, want := range cloud {
		got, ok := state[k]
		if !ok {
			return fmt.Errorf("%s missing key %q in state", attr, k)
		}
		if got != want {
			return fmt.Errorf("%s[%q]: got %q, want %q", attr, k, got, want)
		}
	}
	for k := range state {
		if _, ok := cloud[k]; !ok {
			return fmt.Errorf("%s unexpected key %q in state", attr, k)
		}
	}
	return nil
}

func testAccUserDataSourceConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q

  tags = {
    Name = %[1]q
    Key1 = "Value1"
  }
}

data "ctyun_iam_user" "test" {
  user_name = ctyun_iam_user.test.name
}
`, name)
}

func testAccUserDataSourceConfig_notFound(name string) string {
	return fmt.Sprintf(`
data "ctyun_iam_user" "test" {
  user_name = %[1]q
}
`, name)
}

func testAccUserDataSourceConfig_otherAccountUser(userName string) string {
	return fmt.Sprintf(`
# The cross-account user is created with the second account credentials (CTYUNOOS_ACCESS_KEY_SECOND) and destroyed when the test ends.
data "ctyun_iam_user" "test" {
  user_name = %[1]q
}
`, userName)
}

func testAccUserDataSourceConfig_emptyState() string {
	return `
# intentionally empty: assert failed user data source read left no residual state
`
}

func testAccCheckUserDataSourceAbsentFromState() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.ctyun_iam_user.") {
				continue
			}
			return fmt.Errorf("%s residual in state after failed read (id=%q user_name=%q arn=%q)",
				name,
				rs.Primary.ID,
				rs.Primary.Attributes[names.AttrUserName],
				rs.Primary.Attributes[names.AttrARN],
			)
		}
		return nil
	}
}

func testAccUserDataSourceConfig_lastLogin(userName string) string {
	return fmt.Sprintf(`
data "ctyun_iam_user" "test" {
  user_name = %[1]q
}
`, userName)
}
