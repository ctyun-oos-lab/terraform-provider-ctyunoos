// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package sdkv2

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	terraformsdk "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TC-OOS-PROVIDER-19 AssumeRole SessionName Validation also verifies the default format when session_name is unset.
func TestUnitOOSProviderConfig_AssumeRoleSessionNameDefault(t *testing.T) {
	p, err := NewProvider(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	schema := p.Schema["assume_role"].Elem.(*schema.Resource).Schema["session_name"]
	if schema.DefaultFunc == nil {
		t.Fatal("session_name DefaultFunc is nil")
	}

	value, err := schema.DefaultFunc()
	if err != nil {
		t.Fatal(err)
	}
	name, ok := value.(string)
	if !ok {
		t.Fatalf("session_name default type = %T, want string", value)
	}
	parts := strings.Split(name, "-")
	if len(parts) != 3 || parts[0] != "terraform" || parts[1] != "oos" {
		t.Fatalf("session_name default = %q, want terraform-oos-<UnixNano>", name)
	}
	if len(parts[2]) != 19 {
		t.Fatalf("session_name timestamp length = %d, want 19", len(parts[2]))
	}
	if _, err := strconv.ParseInt(parts[2], 10, 64); err != nil {
		t.Fatalf("session_name timestamp = %q is not numeric: %s", parts[2], err)
	}
}

// TC-OOS-PROVIDER-21 AssumeRole ExternalID Validation verifies external_id length and character-set boundaries.
// TestUnitOOSProviderConfig_AssumeRoleExternalIDValidation verifies external_id length and character-set boundaries.
func TestUnitOOSProviderConfig_AssumeRoleExternalIDValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "length lower bound", value: "ab"},
		{name: "length upper bound", value: strings.Repeat("a", 1224)},
		{name: "length below lower bound", value: "a", wantError: true},
		{name: "length above upper bound", value: strings.Repeat("a", 1225), wantError: true},
		{name: "allowed special characters", value: "ext_id/:test-"},
		{name: "space", value: "external id", wantError: true},
		{name: "exclamation mark", value: "external!id", wantError: true},
		{name: "hash", value: "external#id", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRoleExternalID(tc.value, "external_id")
			if gotError := len(errs) > 0; gotError != tc.wantError {
				t.Fatalf("validation errors = %v, want error = %t", errs, tc.wantError)
			}
		})
	}
}

// TC-OOS-PROVIDER-22 AssumeRole Policy Validation verifies policy length and JSON-format boundaries.
// TestUnitOOSProviderConfig_AssumeRolePolicyValidation verifies policy length and JSON-format boundaries.
func TestUnitOOSProviderConfig_AssumeRolePolicyValidation(t *testing.T) {
	t.Parallel()

	jsonWithLength := func(length int) string {
		const prefix = `{"Statement":"`
		const suffix = `"}`
		return prefix + strings.Repeat("a", length-len(prefix)-len(suffix)) + suffix
	}

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "empty string", value: "", wantError: true},
		{name: "minimal JSON object", value: `{}`},
		{name: "maximum length valid JSON", value: jsonWithLength(2048)},
		{name: "over maximum length", value: jsonWithLength(2048) + "a", wantError: true},
		{name: "valid JSON", value: `{"Version":"2012-10-17","Statement":[]}`},
		{name: "missing property quotes", value: `{Version: 2012}`, wantError: true},
		{name: "missing closing brace", value: `{"Version":"2012-10-17"`, wantError: true},
		{name: "plain text", value: "not-json", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRolePolicy(tc.value, "policy")
			if gotError := len(errs) > 0; gotError != tc.wantError {
				t.Fatalf("validation errors = %v, want error = %t", errs, tc.wantError)
			}
		})
	}
}

// TC-OOS-PROVIDER-23 ExpandAssumeRole Full Fields verifies that every AssumeRole field expands correctly.
// TestUnitOOSProviderConfig_AssumeRoleFullFields verifies the complete field mapping of expandAssumeRole.
func TestUnitOOSProviderConfig_AssumeRoleFullFields(t *testing.T) {
	t.Parallel()

	got, diags := expandAssumeRole(t.Context(), nil, map[string]any{
		"role_arn":        "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
		"session_name":    "tf-session",
		"duration":        "3600s",
		"external_id":     "external-id",
		"policy":          `{"Version":"2012-10-17","Statement":[]}`,
		"source_identity": "source-user",
	})
	if diags.HasError() {
		t.Fatalf("expanding full AssumeRole configuration: %v", diags)
	}

	want := awsbase.AssumeRole{
		RoleARN:        "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
		SessionName:    "tf-session",
		Duration:       time.Hour,
		ExternalID:     "external-id",
		Policy:         `{"Version":"2012-10-17","Statement":[]}`,
		SourceIdentity: "source-user",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded AssumeRole = %#v, want %#v", got, want)
	}
}

// TC-OOS-PROVIDER-24 ExpandAssumeRole Duration Parse verifies duration parsing by expandAssumeRole.
// TestUnitOOSProviderConfig_AssumeRoleDurationParse verifies expansion of unit-bearing values and behavior for invalid values that bypass the schema.
func TestUnitOOSProviderConfig_AssumeRoleDurationParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		duration string
		want     time.Duration
	}{
		{name: "Go duration", duration: "1h", want: time.Hour},
		{name: "duration seconds with unit", duration: "3600s", want: time.Hour},
		{name: "bare seconds bypassing schema", duration: "3600", want: 0},
		{name: "invalid duration bypassing schema", duration: "abc", want: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := expandAssumeRole(t.Context(), nil, map[string]any{
				"role_arn": "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
				"duration": tc.duration,
			})
			if diags.HasError() {
				t.Fatalf("expanding duration %q: %v", tc.duration, diags)
			}
			if got.Duration != tc.want {
				t.Fatalf("duration %q expanded to %s, want %s", tc.duration, got.Duration, tc.want)
			}
		})
	}
}

// TC-OOS-PROVIDER-20 AssumeRole SourceIdentity Validation verifies source_identity length and character-set boundaries.
// TestUnitOOSProviderConfig_AssumeRoleSourceIdentityValidation verifies source_identity length and character-set boundaries.
func TestUnitOOSProviderConfig_AssumeRoleSourceIdentityValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "length lower bound", value: "ab"},
		{name: "length upper bound", value: strings.Repeat("a", 64)},
		{name: "length below lower bound", value: "a", wantError: true},
		{name: "length above upper bound", value: strings.Repeat("a", 65), wantError: true},
		{name: "allowed special characters", value: "test_session.name+@-="},
		{name: "space", value: "test session", wantError: true},
		{name: "exclamation mark", value: "test!session", wantError: true},
		{name: "Chinese characters", value: "测试会话", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRoleSourceIdentity(tc.value, "source_identity")
			if gotError := len(errs) > 0; gotError != tc.wantError {
				t.Fatalf("validation errors = %v, want error = %t", errs, tc.wantError)
			}
		})
	}
}

// TC-OOS-PROVIDER-19 AssumeRole SessionName Validation verifies session_name length and character-set boundaries.
// TestUnitOOSProviderConfig_AssumeRoleSessionNameValidation verifies session_name length and character-set boundaries.
func TestUnitOOSProviderConfig_AssumeRoleSessionNameValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "length lower bound", value: "ab"},
		{name: "length upper bound", value: strings.Repeat("a", 64)},
		{name: "length below lower bound", value: "a", wantError: true},
		{name: "length above upper bound", value: strings.Repeat("a", 65), wantError: true},
		{name: "allowed special characters", value: "test_session.name+@-="},
		{name: "space", value: "test session", wantError: true},
		{name: "exclamation mark", value: "test!session", wantError: true},
		{name: "Chinese characters", value: "测试会话", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRoleSessionName(tc.value, "session_name")
			if gotError := len(errs) > 0; gotError != tc.wantError {
				t.Fatalf("validation errors = %v, want error = %t", errs, tc.wantError)
			}
		})
	}
}

// TC-OOS-PROVIDER-17 AssumeRole Duration Validation verifies assume_role.duration format and range validation.
// TestUnitOOSProviderConfig_AssumeRoleDurationValidation verifies assume_role.duration format and range validation.
func TestUnitOOSProviderConfig_AssumeRoleDurationValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		val         any
		expectedErr *regexp.Regexp
	}{
		{
			name:        "empty string",
			val:         "",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name:        "bare lower bound",
			val:         "900",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name:        "bare below lower bound",
			val:         "899",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name:        "bare upper bound",
			val:         "43200",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name:        "bare above upper bound",
			val:         "43201",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name: "lower bound seconds",
			val:  "900s",
		},
		{
			name:        "below lower bound seconds",
			val:         "899s",
			expectedErr: regexache.MustCompile(`must be between 15 minutes \(15m\) and 12 hours \(12h\)`),
		},
		{
			name: "upper bound seconds",
			val:  "43200s",
		},
		{
			name:        "above upper bound seconds",
			val:         "43201s",
			expectedErr: regexache.MustCompile(`must be between 15 minutes \(15m\) and 12 hours \(12h\)`),
		},
		{
			name:        "below lower bound minutes",
			val:         "10m",
			expectedErr: regexache.MustCompile(`must be between 15 minutes \(15m\) and 12 hours \(12h\)`),
		},
		{
			name:        "above upper bound duration",
			val:         "12h30m",
			expectedErr: regexache.MustCompile(`must be between 15 minutes \(15m\) and 12 hours \(12h\)`),
		},
		{
			name: "lower bound minutes",
			val:  "15m",
		},
		{
			name:        "invalid duration",
			val:         "abc",
			expectedErr: regexache.MustCompile(`cannot be parsed as a duration`),
		},
		{
			name: "middle duration",
			val:  "1h10m10s",
		},
		{
			name: "upper bound hours",
			val:  "12h",
		},
	}

	matchErr := func(errs []error, r *regexp.Regexp) bool {
		for _, err := range errs {
			if r.MatchString(err.Error()) {
				return true
			}
		}

		return false
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRoleDuration(tc.val, "test_property")

			if len(errs) == 0 && tc.expectedErr == nil {
				return
			}

			if len(errs) != 0 && tc.expectedErr == nil {
				t.Fatalf("expected no validation errors, got %v", errs)
			}

			if !matchErr(errs, tc.expectedErr) {
				t.Fatalf("expected error matching %q, got %v", tc.expectedErr, errs)
			}
		})
	}
}

// TC-OOS-PROVIDER-17 AssumeRole Duration Validation verifies that no duration value is sent when duration is unset.
// TestUnitOOSProviderConfig_AssumeRoleDurationOmitted verifies that no duration value is sent when duration is unset.
func TestUnitOOSProviderConfig_AssumeRoleDurationOmitted(t *testing.T) {
	result, diags := expandAssumeRole(t.Context(), nil, map[string]any{
		"role_arn": "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
	})
	if diags.HasError() {
		t.Fatalf("expanding assume_role without duration: %v", diags)
	}
	if result.Duration != 0 {
		t.Fatalf("duration = %s, want zero when omitted", result.Duration)
	}
}

// TC-OOS-PROVIDER-17 AssumeRole Duration Validation verifies that duration validation is wired into the Terraform provider schema.
// TestUnitOOSProviderConfig_AssumeRoleDurationSchemaValidation verifies valid values pass and invalid values fail during provider.Validate.
func TestUnitOOSProviderConfig_AssumeRoleDurationSchemaValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		duration  string
		wantError bool
	}{
		{name: "valid Go duration", duration: "3600s"},
		{name: "bare seconds rejected by schema", duration: "3600", wantError: true},
		{name: "invalid duration rejected by schema", duration: "abc", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			config := map[string]any{
				"access_key": "StaticAccessKey",
				"secret_key": "StaticSecretKey",
				"endpoints":  oosEndpoints(),
				"assume_role": []any{map[string]any{
					"role_arn": "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
					"duration": tc.duration,
				}},
			}

			diags := p.Validate(terraformsdk.NewResourceConfigRaw(config))
			if gotError := diags.HasError(); gotError != tc.wantError {
				t.Fatalf("provider schema validation error = %t, want %t; diagnostics: %v", gotError, tc.wantError, diags)
			}
		})
	}
}

// TC-OOS-PROVIDER-18/19/20/21/22 AssumeRole Schema Validation verifies that field validators are wired into the provider schema.
func TestUnitOOSProviderConfig_AssumeRoleSchemaValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		field     string
		value     string
		wantNoErr bool
	}{
		{name: "role ARN", field: "role_arn", value: "not-an-arn"},
		{name: "session name", field: "session_name", value: "x"},
		{name: "source identity", field: "source_identity", value: "x"},
		{name: "external ID", field: "external_id", value: "x"},
		{name: "policy", field: "policy", value: "not-json"},
		{name: "valid role ARN", field: "role_arn", value: "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole", wantNoErr: true},
		{name: "valid session name", field: "session_name", value: "valid-session", wantNoErr: true},
		{name: "valid source identity", field: "source_identity", value: "valid-source", wantNoErr: true},
		{name: "valid external ID", field: "external_id", value: "valid-external", wantNoErr: true},
		{name: "valid policy", field: "policy", value: `{}`, wantNoErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProvider(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			assumeRole := map[string]any{
				"role_arn":        "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
				"session_name":    "valid-session",
				"source_identity": "valid-source",
				"external_id":     "valid-external",
				"policy":          `{}`,
			}
			assumeRole[tc.field] = tc.value
			config := map[string]any{
				"access_key":  "StaticAccessKey",
				"secret_key":  "StaticSecretKey",
				"endpoints":   oosEndpoints(),
				"assume_role": []any{assumeRole},
			}
			diags := p.Validate(terraformsdk.NewResourceConfigRaw(config))
			if got := !diags.HasError(); got != tc.wantNoErr {
				t.Fatalf("schema validation success = %t, want %t; diagnostics: %v", got, tc.wantNoErr, diags)
			}
		})
	}
}

// TC-OOS-PROVIDER-25 ExpandAssumeRole Missing RoleARN verifies that a missing role_arn returns RequiredAttributeError pointing to role_arn.
func TestUnitOOSProviderConfig_AssumeRoleMissingRoleARN(t *testing.T) {
	t.Parallel()

	roleARNPath := cty.GetAttrPath("role_arn")
	testCases := []struct {
		name   string
		config map[string]any
	}{
		{name: "missing role_arn", config: map[string]any{}},
		{name: "empty role_arn", config: map[string]any{"role_arn": ""}},
		{name: "null role_arn", config: map[string]any{"role_arn": nil}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := expandAssumeRole(t.Context(), roleARNPath, tc.config)
			if len(diags) != 1 || !diags.HasError() {
				t.Fatalf("diagnostics = %v, want one error diagnostic", diags)
			}

			diagnostic := diags[0]
			if diagnostic.Summary != "Missing required argument" {
				t.Fatalf("diagnostic summary = %q, want %q", diagnostic.Summary, "Missing required argument")
			}
			if !reflect.DeepEqual(diagnostic.AttributePath, roleARNPath) {
				t.Fatalf("diagnostic path = %#v, want %#v", diagnostic.AttributePath, roleARNPath)
			}
			if !strings.Contains(diagnostic.Detail, `"role_arn"`) {
				t.Fatalf("diagnostic detail = %q, want role_arn", diagnostic.Detail)
			}
		})
	}
}

// TC-OOS-PROVIDER-13 AssumeRole Role Chain verifies expansion of multiple assume_role blocks and error locations.
// TestUnitOOSProviderConfig_AssumeRolesExpansion verifies multi-block mapping by expandAssumeRoles and diagnostics for invalid elements.
func TestUnitOOSProviderConfig_AssumeRolesExpansion(t *testing.T) {
	t.Parallel()

	path := cty.GetAttrPath("assume_role")
	firstRoleARN := "arn:ctyun:iam::32fefj64y54gc:role/AssumeRoleA"
	secondRoleARN := "arn:ctyun:iam::32fefj64y54gc:role/AssumeRoleB"

	t.Run("multiple blocks", func(t *testing.T) {
		got, diags := expandAssumeRoles(t.Context(), path, []any{
			map[string]any{
				"role_arn":     firstRoleARN,
				"session_name": "role-a-session",
			},
			map[string]any{
				"role_arn":        secondRoleARN,
				"external_id":     "role-b-external-id",
				"source_identity": "role-b-source",
			},
		})
		if diags.HasError() {
			t.Fatalf("expanding multiple AssumeRole blocks: %v", diags)
		}

		want := []awsbase.AssumeRole{
			{RoleARN: firstRoleARN, SessionName: "role-a-session"},
			{RoleARN: secondRoleARN, ExternalID: "role-b-external-id", SourceIdentity: "role-b-source"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expanded AssumeRole blocks = %#v, want %#v", got, want)
		}
	})

	t.Run("invalid list element", func(t *testing.T) {
		got, diags := expandAssumeRoles(t.Context(), path, []any{
			map[string]any{"role_arn": firstRoleARN},
			"not-an-assume-role-block",
			map[string]any{"role_arn": secondRoleARN},
		})
		if len(diags) != 1 || !diags.HasError() {
			t.Fatalf("diagnostics = %v, want one error diagnostic", diags)
		}
		if got[0].RoleARN != firstRoleARN {
			t.Fatalf("first expanded role ARN = %q, want %q", got[0].RoleARN, firstRoleARN)
		}

		diagnostic := diags[0]
		wantPath := path.IndexInt(1)
		if diagnostic.Summary != "Missing required argument" {
			t.Fatalf("diagnostic summary = %q, want %q", diagnostic.Summary, "Missing required argument")
		}
		if !reflect.DeepEqual(diagnostic.AttributePath, wantPath) {
			t.Fatalf("diagnostic path = %#v, want %#v", diagnostic.AttributePath, wantPath)
		}
		if !strings.Contains(diagnostic.Detail, `"role_arn"`) {
			t.Fatalf("diagnostic detail = %q, want role_arn", diagnostic.Detail)
		}
	})
}

// TC-OOS-PROVIDER-18 AssumeRole RoleARN Validation verifies ARN-format validation for assume_role.role_arn.
// TestUnitOOSProviderConfig_AssumeRoleRoleARNValidation verifies ARN-format validation for assume_role.role_arn.
func TestUnitOOSProviderConfig_AssumeRoleRoleARNValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		val          any
		expectedErrs []*regexp.Regexp
	}{
		{
			name: "empty",
			val:  "",
		},
		{
			name: "valid",
			val:  "arn:ctyun:iam::32fefj64y54gc:role/AssumeRole",
		},
		{
			name: "invalid CTYUN account ID",
			val:  "arn:ctyun:iam::123:role/AssumeRole",
			expectedErrs: []*regexp.Regexp{
				regexache.MustCompile(regexp.QuoteMeta(`"test_property" (arn:ctyun:iam::123:role/AssumeRole) is an invalid ARN: invalid account ID value (expecting to match regular expression: ^[0-9a-z]{13}$)`)),
			},
		},
		{
			name: "aws account id",
			val:  "arn:aws:iam::555555555555:role/AssumeRole",
			expectedErrs: []*regexp.Regexp{
				regexache.MustCompile(regexp.QuoteMeta(`"test_property" (arn:aws:iam::555555555555:role/AssumeRole) is an invalid ARN: invalid account ID value (expecting to match regular expression: ^[0-9a-z]{13}$)`)),
				regexache.MustCompile(regexp.QuoteMeta(`"test_property" must be a CTYUN IAM role ARN`)),
			},
		},
		{
			name:         "policy arn",
			val:          "arn:ctyun:iam::32fefj64y54gc:policy/AssumeRole",
			expectedErrs: []*regexp.Regexp{regexache.MustCompile(`must be an IAM role ARN`)},
		},
		{
			name: "not an arn",
			val:  "not-an-arn",
			expectedErrs: []*regexp.Regexp{
				regexache.MustCompile(regexp.QuoteMeta(`expected length of test_property to be in the range (20 - 2048), got not-an-arn`)),
				regexache.MustCompile(regexp.QuoteMeta(`"test_property" (not-an-arn) is an invalid ARN: arn: invalid prefix`)),
			},
		},
		{
			name: "too short",
			val:  "arn:ctyun:iam::",
			expectedErrs: []*regexp.Regexp{
				regexache.MustCompile(regexp.QuoteMeta(`expected length of test_property to be in the range (20 - 2048), got arn:ctyun:iam::`)),
				regexache.MustCompile(regexp.QuoteMeta(`"test_property" (arn:ctyun:iam::) is an invalid ARN: arn: not enough sections`)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validAssumeRoleRoleARN(tc.val, "test_property")

			if len(errs) == 0 && len(tc.expectedErrs) == 0 {
				return
			}

			if len(errs) != len(tc.expectedErrs) {
				t.Fatalf("expected %d validation errors, got %d: %v", len(tc.expectedErrs), len(errs), errs)
			}

			for _, expectedErr := range tc.expectedErrs {
				matched := false
				for _, err := range errs {
					if expectedErr.MatchString(err.Error()) {
						matched = true
						break
					}
				}
				if !matched {
					t.Fatalf("expected an error matching %q, got %v", expectedErr, errs)
				}
			}
		})
	}
}

// TC-OOS-PROVIDER-18 AssumeRole RoleARN Validation verifies validation of partition, service, and resource type in parsed ARNs.
// TestUnitOOSProviderConfig_AssumeRoleARNValidation directly covers every decision branch in validAssumeRoleARN.
func TestUnitOOSProviderConfig_AssumeRoleARNValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		parsedARN  arn.ARN
		wantErrors []string
	}{
		{
			name: "valid CTYUN IAM role",
			parsedARN: arn.ARN{
				Partition: "ctyun",
				Service:   "iam",
				Resource:  "role/AssumeRole",
			},
		},
		{
			name: "non CTYUN partition",
			parsedARN: arn.ARN{
				Partition: "aws",
				Service:   "iam",
				Resource:  "role/AssumeRole",
			},
			wantErrors: []string{`"test_property" must be a CTYUN IAM role ARN`},
		},
		{
			name: "non IAM service",
			parsedARN: arn.ARN{
				Partition: "ctyun",
				Service:   "s3",
				Resource:  "role/AssumeRole",
			},
			wantErrors: []string{`"test_property" must be an IAM role ARN`},
		},
		{
			name: "non role resource",
			parsedARN: arn.ARN{
				Partition: "ctyun",
				Service:   "iam",
				Resource:  "user/AssumeRole",
			},
			wantErrors: []string{`"test_property" must be an IAM role ARN`},
		},
		{
			name: "non CTYUN non IAM ARN",
			parsedARN: arn.ARN{
				Partition: "aws",
				Service:   "s3",
				Resource:  "bucket/example",
			},
			wantErrors: []string{
				`"test_property" must be a CTYUN IAM role ARN`,
				`"test_property" must be an IAM role ARN`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			warnings, errs := validAssumeRoleARN(nil, "test_property", tc.parsedARN)
			if len(warnings) != 0 {
				t.Fatalf("validation warnings = %v, want none", warnings)
			}
			if len(errs) != len(tc.wantErrors) {
				t.Fatalf("validation errors = %v, want %d errors", errs, len(tc.wantErrors))
			}
			for i, wantError := range tc.wantErrors {
				if errs[i].Error() != wantError {
					t.Fatalf("validation error %d = %q, want %q", i, errs[i], wantError)
				}
			}
		})
	}
}
