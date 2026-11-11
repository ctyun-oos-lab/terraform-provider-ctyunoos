// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"testing"

	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
)

func TestUnitS3BucketObjectLockConfigurationDataSource_FlattenRule(t *testing.T) {
	ctx := context.Background()
	days := int32(7)
	years := int32(2)
	wantDays := int64(days)
	wantYears := int64(years)

	testCases := []struct {
		name      string
		rule      *tfs3.OOSLockRule
		wantNull  bool
		wantDays  *int64
		wantYears *int64
	}{
		{
			name:     "nil-rule",
			wantNull: true,
		},
		{
			name: "nil-default-retention",
			rule: &tfs3.OOSLockRule{
				DefaultRetention: nil,
			},
			wantNull: true,
		},
		{
			name: "days-only",
			rule: &tfs3.OOSLockRule{
				DefaultRetention: &tfs3.OOSLockDefaultRetention{
					Mode: "COMPLIANCE",
					Days: &days,
				},
			},
			wantDays: &wantDays,
		},
		{
			name: "years-only",
			rule: &tfs3.OOSLockRule{
				DefaultRetention: &tfs3.OOSLockDefaultRetention{
					Mode:  "COMPLIANCE",
					Years: &years,
				},
			},
			wantYears: &wantYears,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			flattened := tfs3.FlattenOOSObjectLockConfigDataSourceRule(ctx, testCase.rule)
			if got := flattened.IsNull(); got != testCase.wantNull {
				t.Fatalf("flattened rule null = %t, want %t", got, testCase.wantNull)
			}
			if testCase.wantNull {
				return
			}

			rule, diags := flattened.ToPtr(ctx)
			if diags.HasError() {
				t.Fatalf("flattened rule returned diagnostics: %v", diags.Errors())
			}
			retention, diags := rule.DefaultRetention.ToPtr(ctx)
			if diags.HasError() {
				t.Fatalf("flattened default retention returned diagnostics: %v", diags.Errors())
			}

			if got := retention.Mode.ValueString(); got != "COMPLIANCE" {
				t.Errorf("mode = %q, want %q", got, "COMPLIANCE")
			}
			testS3BucketObjectLockConfigurationDataSourceCheckOptionalInt64(t, "days", retention.Days.IsNull(), retention.Days.ValueInt64(), testCase.wantDays)
			testS3BucketObjectLockConfigurationDataSourceCheckOptionalInt64(t, "years", retention.Years.IsNull(), retention.Years.ValueInt64(), testCase.wantYears)
		})
	}
}

func testS3BucketObjectLockConfigurationDataSourceCheckOptionalInt64(t *testing.T, name string, isNull bool, got int64, want *int64) {
	t.Helper()

	if want == nil {
		if !isNull {
			t.Errorf("%s null = false, want true", name)
		}
		return
	}
	if isNull {
		t.Errorf("%s null = true, want false", name)
		return
	}
	if got != *want {
		t.Errorf("%s = %d, want %d", name, got, *want)
	}
}
