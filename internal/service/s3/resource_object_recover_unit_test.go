// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TC-S3-OBJECT-RECOVER-08 Input Validation
// TestUnitS3ObjectRecover_ExpandObjectRecoverInput verifies that recover, after-only, before-only, and time-window configurations are stably expanded into OOS recover request parameters.
func TestUnitS3ObjectRecover_ExpandObjectRecoverInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input map[string]any
		want  oosRecoverObjectsInput
	}{
		{
			name: "recover mode",
			input: map[string]any{
				"bucket":        "test-bucket",
				"encoding_type": "url",
				"recover":       []any{map[string]any{"key": "key1"}, map[string]any{"key": "dir/key2"}},
			},
			want: oosRecoverObjectsInput{
				bucket:       "test-bucket",
				encodingType: "url",
				keys:         []string{"key1", "dir/key2"},
			},
		},
		{
			name: "after only",
			input: map[string]any{
				"bucket":            "test-bucket",
				"soft_delete_after": 1769158000,
			},
			want: oosRecoverObjectsInput{
				bucket:          "test-bucket",
				softDeleteAfter: "1769158000",
			},
		},
		{
			name: "before only",
			input: map[string]any{
				"bucket":             "test-bucket",
				"soft_delete_before": 1769750800,
			},
			want: oosRecoverObjectsInput{
				bucket:           "test-bucket",
				softDeleteBefore: "1769750800",
			},
		},
		{
			name: "window range",
			input: map[string]any{
				"bucket":             "test-bucket",
				"soft_delete_after":  1769158000,
				"soft_delete_before": 1769750800,
			},
			want: oosRecoverObjectsInput{
				bucket:           "test-bucket",
				softDeleteAfter:  "1769158000",
				softDeleteBefore: "1769750800",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := schema.TestResourceDataRaw(t, resourceObjectRecover().Schema, tc.input)
			got := expandObjectRecoverInput(d)

			if got.bucket != tc.want.bucket {
				t.Fatalf("got bucket %q, want %q", got.bucket, tc.want.bucket)
			}
			if got.softDeleteAfter != tc.want.softDeleteAfter {
				t.Fatalf("got softDeleteAfter %q, want %q", got.softDeleteAfter, tc.want.softDeleteAfter)
			}
			if got.softDeleteBefore != tc.want.softDeleteBefore {
				t.Fatalf("got softDeleteBefore %q, want %q", got.softDeleteBefore, tc.want.softDeleteBefore)
			}
			if got.encodingType != tc.want.encodingType {
				t.Fatalf("got encodingType %q, want %q", got.encodingType, tc.want.encodingType)
			}

			if !reflect.DeepEqual(got.keys, tc.want.keys) {
				t.Fatalf("got keys %#v, want %#v", got.keys, tc.want.keys)
			}
		})
	}
}

func TestUnitS3ObjectRecover_ExpandObjectRecoverKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		items []any
		want  []string
	}{
		{
			name:  "empty key",
			items: []any{map[string]any{"key": ""}},
			want:  []string{""},
		},
		{
			name:  "non map element",
			items: []any{"not-a-map"},
			want:  nil,
		},
		{
			name: "mixed elements",
			items: []any{
				map[string]any{"key": "key-1"},
				"not-a-map",
				map[string]any{"key": "key-2"},
			},
			want: []string{"key-1", "key-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := expandObjectRecoverKeys(tc.items); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got keys %#v, want %#v", got, tc.want)
			}
		})
	}
}
func TestUnitS3ObjectRecover_DecodeRecoverObjectsResult(t *testing.T) {
	result := &oosRecoverObjectsResult{
		Recovered: []oosRecoverObject{{Key: "test%2Fobject+one%2Bplus"}},
		RecoverError: []oosRecoverError{{
			Key: "missing%2Fobject%20two",
		}},
	}

	if err := decodeRecoverObjectsResult(result, "url"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := result.Recovered[0].Key, "test/object one+plus"; got != want {
		t.Fatalf("got recovered key %q, want %q", got, want)
	}
	if got, want := result.RecoverError[0].Key, "missing/object two"; got != want {
		t.Fatalf("got error key %q, want %q", got, want)
	}
}

func TestUnitS3ObjectRecover_DecodeRecoverObjectsResultNilResult(t *testing.T) {
	t.Parallel()

	if err := decodeRecoverObjectsResult(nil, "url"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestUnitS3ObjectRecover_DecodeRecoverObjectsResultEmptyEncodingType(t *testing.T) {
	t.Parallel()

	result := &oosRecoverObjectsResult{
		Recovered:    []oosRecoverObject{{Key: "test%2Fobject"}},
		RecoverError: []oosRecoverError{{Key: "missing%2Fobject"}},
	}

	if err := decodeRecoverObjectsResult(result, ""); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := result.Recovered[0].Key, "test%2Fobject"; got != want {
		t.Fatalf("got recovered key %q, want %q", got, want)
	}
	if got, want := result.RecoverError[0].Key, "missing%2Fobject"; got != want {
		t.Fatalf("got error key %q, want %q", got, want)
	}
}

func TestUnitS3ObjectRecover_DecodeRecoverObjectsResultRecoverErrorIncompleteURLPath(t *testing.T) {
	t.Parallel()

	result := &oosRecoverObjectsResult{
		RecoverError: []oosRecoverError{{Key: "invalid%"}},
	}

	err := decodeRecoverObjectsResult(result, "url")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "unescaping recover error key"; !strings.Contains(got, want) {
		t.Fatalf("got error %q, want it to contain %q", got, want)
	}
}
func TestUnitS3ObjectRecover_FlattenRecoveredObjectKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		result *oosRecoverObjectsResult
		want   []string
	}{
		{
			name: "nil result",
			want: nil,
		},
		{
			name:   "empty recovered",
			result: &oosRecoverObjectsResult{},
			want:   []string{},
		},
		{
			name: "multiple keys",
			result: &oosRecoverObjectsResult{
				Recovered: []oosRecoverObject{{Key: "a"}, {Key: "b"}},
			},
			want: []string{"a", "b"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := flattenRecoveredObjectKeys(tc.result); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestUnitS3ObjectRecover_FlattenRecoverErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		errors []oosRecoverError
		want   []map[string]any
	}{
		{
			name:   "nil errors",
			errors: nil,
			want:   []map[string]any{},
		},
		{
			name:   "empty code and message",
			errors: []oosRecoverError{{Key: "k"}},
			want: []map[string]any{
				{names.AttrKey: "k", "code": "", names.AttrMessage: ""},
			},
		},
		{
			name: "multiple errors",
			errors: []oosRecoverError{
				{Key: "key-1", Code: "NoSuchKey", Message: "not found"},
				{Key: "key-2", Code: "AccessDenied", Message: "denied"},
			},
			want: []map[string]any{
				{names.AttrKey: "key-1", "code": "NoSuchKey", names.AttrMessage: "not found"},
				{names.AttrKey: "key-2", "code": "AccessDenied", names.AttrMessage: "denied"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := flattenRecoverErrors(tc.errors); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TC-S3-OBJECT-RECOVER-08 Input Validation - incomplete URL escape
// TestUnitS3ObjectRecover_DecodeRecoverObjectsResultIncompleteURLPath verifies that an incomplete URL escape returned with encoding_type=url yields a clear error.
func TestUnitS3ObjectRecover_DecodeRecoverObjectsResultIncompleteURLPath(t *testing.T) {
	result := &oosRecoverObjectsResult{
		Recovered: []oosRecoverObject{{Key: "invalid%"}},
	}

	err := decodeRecoverObjectsResult(result, "url")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got, want := err.Error(), "invalid URL escape"; !strings.Contains(got, want) {
		t.Fatalf("got error %q, want it to contain %q", got, want)
	}
}

// TC-S3-OBJECT-RECOVER-07 Invalid Time Window
// TestUnitS3ObjectRecover_ValidateObjectRecoverTimeWindow verifies a stable error when soft_delete_before is earlier than soft_delete_after, and covers valid selector combinations.
func TestUnitS3ObjectRecover_ValidateObjectRecoverTimeWindow(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		after, before       int
		hasAfter, hasBefore bool
		wantError           string
	}{
		{
			name: "recover mode",
		},
		{
			name:     "after only",
			after:    1769158000,
			hasAfter: true,
		},
		{
			name:      "before only",
			before:    1769750800,
			hasBefore: true,
		},
		{
			name:      "time window",
			after:     1769158000,
			before:    1769750800,
			hasAfter:  true,
			hasBefore: true,
		},
		{
			name:      "invalid time window",
			after:     1769750800,
			before:    1769158000,
			hasAfter:  true,
			hasBefore: true,
			wantError: "soft_delete_before must be greater than or equal to soft_delete_after",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateObjectRecoverTimeWindow(tc.after, tc.before, tc.hasAfter, tc.hasBefore)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantError {
				t.Fatalf("got error %v, want %q", err, tc.wantError)
			}
		})
	}
}

// TestUnitS3ObjectRecover_ObjectRecoverID verifies that identical recover inputs produce a stable ID and different inputs produce different IDs.
func TestUnitS3ObjectRecover_ObjectRecoverID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		a    oosRecoverObjectsInput
		b    oosRecoverObjectsInput
		eq   bool
	}{
		{
			name: "same input same id",
			a: oosRecoverObjectsInput{
				bucket:          "test-bucket",
				softDeleteAfter: "1769158000",
				keys:            []string{"key1", "dir/key2"},
			},
			b: oosRecoverObjectsInput{
				bucket:          "test-bucket",
				softDeleteAfter: "1769158000",
				keys:            []string{"key1", "dir/key2"},
			},
			eq: true,
		},
		{
			name: "different selector different id",
			a: oosRecoverObjectsInput{
				bucket:          "test-bucket",
				softDeleteAfter: "1769158000",
			},
			b: oosRecoverObjectsInput{
				bucket:           "test-bucket",
				softDeleteBefore: "1769158000",
			},
			eq: false,
		},
		{
			name: "different key order different id",
			a: oosRecoverObjectsInput{
				bucket: "test-bucket",
				keys:   []string{"key1", "key2"},
			},
			b: oosRecoverObjectsInput{
				bucket: "test-bucket",
				keys:   []string{"key2", "key1"},
			},
			eq: false,
		},
		{
			name: "different encoding type different id",
			a: oosRecoverObjectsInput{
				bucket:       "test-bucket",
				keys:         []string{"key1"},
				encodingType: "url",
			},
			b: oosRecoverObjectsInput{
				bucket: "test-bucket",
				keys:   []string{"key1"},
			},
			eq: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			idA := objectRecoverID(tc.a)
			idB := objectRecoverID(tc.b)
			if tc.eq {
				if idA != idB {
					t.Fatalf("got ids %q and %q, want equal", idA, idB)
				}
				return
			}
			if idA == idB {
				t.Fatalf("got identical ids %q and %q, want different", idA, idB)
			}
		})
	}
}

func TestUnitFormatRecoverErrors(t *testing.T) {
	t.Parallel()

	errs := []oosRecoverError{
		{Key: "key1", Code: "AccessDenied", Message: "AccessDenied"},
		{Key: "key2", Code: "NoSuchKey", Message: "Not in soft delete retention"},
	}

	if got, want := formatRecoverErrors(errs), "key1: AccessDenied (AccessDenied); key2: Not in soft delete retention (NoSuchKey)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
