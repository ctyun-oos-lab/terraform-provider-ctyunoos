// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"testing"
)

func TestUnitS3ObjectDataSource_IsContentTypeAllowed(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		contentType string
		nilInput    bool
		want        bool
	}{
		"nil":                      {nilInput: true},
		"empty":                    {contentType: ""},
		"application/atom+xml":     {contentType: "application/atom+xml", want: true},
		"application/json":         {contentType: "application/json", want: true},
		"application/ld+json":      {contentType: "application/ld+json", want: true},
		"application/x-csh":        {contentType: "application/x-csh", want: true},
		"application/x-httpd-php":  {contentType: "application/x-httpd-php", want: true},
		"application/x-sh":         {contentType: "application/x-sh", want: true},
		"application/xhtml+xml":    {contentType: "application/xhtml+xml", want: true},
		"application/xml":          {contentType: "application/xml", want: true},
		"application/x-sql":        {contentType: "application/x-sql", want: true},
		"application/yaml":         {contentType: "application/yaml", want: true},
		"text/csv":                 {contentType: "text/csv", want: true},
		"application/octet-stream": {contentType: "application/octet-stream"},
		"image/png":                {contentType: "image/png"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var contentType *string
			if !tc.nilInput {
				contentType = &tc.contentType
			}

			if got := isContentTypeAllowed(contentType); got != tc.want {
				t.Errorf("IsContentTypeAllowed(%q) = %t, want %t", tc.contentType, got, tc.want)
			}
		})
	}
}

func TestUnitS3ObjectDataSource_ObjectDeleteMarkerError(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		deleteMarker bool
		wantError    string
	}{
		"delete marker": {
			deleteMarker: true,
			wantError:    "S3 Bucket (test-bucket) Object (test-key) has been deleted",
		},
		"object exists": {},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := objectDeleteMarkerError("test-bucket", "test-key", tc.deleteMarker)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := err.Error(); got != tc.wantError {
				t.Errorf("error = %q, want %q", got, tc.wantError)
			}
		})
	}
}
