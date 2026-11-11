// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"testing"
)

func TestUnitS3BucketLifecycleConfigurationDataSource_NormalizeLifecycleRFC3339(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "utc-midnight",
			input: "2030-01-02T00:00:00.000Z",
			want:  "2030-01-02T00:00:00.000Z",
		},
		{
			name:  "time-is-normalized-to-midnight",
			input: "2030-01-02T12:30:00.000Z",
			want:  "2030-01-02T00:00:00.000Z",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := normalizeLifecycleRFC3339(testCase.input); got != testCase.want {
				t.Fatalf("normalizeLifecycleRFC3339(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestUnitS3BucketLifecycleConfigurationDataSource_FlattenLifecycleConfigDataSourceRules(t *testing.T) {
	ctx := context.Background()
	flattened := flattenLifecycleConfigDataSourceRules(ctx, []oosLifecycleRule{
		{
			ID:     "expiration-date",
			Prefix: "expiration/",
			Status: lifecycleRuleStatusEnabled,
			Expiration: &oosLifecycleExpiry{
				Date: "2030-01-02T12:30:00.000Z",
			},
		},
		{
			ID:     "transition-date",
			Prefix: "transition/",
			Status: lifecycleRuleStatusDisabled,
			Transition: &oosLifecycleTransit{
				Date:         "2031-02-03T23:59:59.000Z",
				StorageClass: "STANDARD_IA",
			},
		},
	})

	rules, diags := flattened.ToSlice(ctx)
	if diags.HasError() {
		t.Fatalf("flattened rules returned diagnostics: %v", diags.Errors())
	}
	if len(rules) != 2 {
		t.Fatalf("flattened rule count = %d, want 2", len(rules))
	}

	expiration, diags := rules[0].Expiration.ToPtr(ctx)
	if diags.HasError() {
		t.Fatalf("expiration returned diagnostics: %v", diags.Errors())
	}
	if got, want := expiration.Date.ValueString(), "2030-01-02T00:00:00.000Z"; got != want {
		t.Errorf("expiration date = %q, want %q", got, want)
	}

	transition, diags := rules[1].Transition.ToPtr(ctx)
	if diags.HasError() {
		t.Fatalf("transition returned diagnostics: %v", diags.Errors())
	}
	if got, want := transition.Date.ValueString(), "2031-02-03T00:00:00.000Z"; got != want {
		t.Errorf("transition date = %q, want %q", got, want)
	}
	if got, want := transition.StorageClass.ValueString(), "STANDARD_IA"; got != want {
		t.Errorf("transition storage class = %q, want %q", got, want)
	}
}
