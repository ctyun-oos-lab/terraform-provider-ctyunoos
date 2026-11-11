// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"encoding/xml"
	"reflect"
	"testing"
)

// TC-OOS-DSD-10 Missing NextMarker: a truncated response with an empty
// NextMarker is the server's end-of-list signal and carries no objects. The
// read loop must break on it instead of re-issuing the request with an empty
// marker (which would re-fetch the first page forever). This verifies the XML
// deserializes as truncated, marker-less, and empty so the loop can detect and
// stop on it without losing data.
func TestUnitS3ObjectsSoftDeleteDataSource_MissingNextMarker(t *testing.T) {
	t.Parallel()

	var result oosListObjectsSoftDeleteResult
	if err := xml.Unmarshal([]byte(`
<ListObjectsSoftDeleteResult>
  <Name>test-bucket</Name>
  <Marker></Marker>
  <NextMarker></NextMarker>
  <MaxKeys>2</MaxKeys>
  <IsTruncated>true</IsTruncated>
</ListObjectsSoftDeleteResult>`), &result); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !result.IsTruncated {
		t.Fatal("expected IsTruncated to be true")
	}
	if result.NextMarker != "" {
		t.Fatalf("expected empty NextMarker, got %q", result.NextMarker)
	}
	if len(result.Contents) != 0 {
		t.Fatalf("expected no contents, got %d", len(result.Contents))
	}
}

func TestUnitS3ObjectsSoftDeleteDataSource_DecodeSoftDeletedObjectKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		key          string
		encodingType string
		want         string
		wantErr      bool
	}{
		{name: "url", key: "acc-soft-delete%2Furl%2Fa+%2B+b", encodingType: "url", want: "acc-soft-delete/url/a + b"},
		{name: "url case insensitive", key: "acc-soft-delete%2Furl%2Fa+%2B+b", encodingType: "URL", want: "acc-soft-delete/url/a + b"},
		{name: "no encoding", key: "acc-soft-delete%2Furl%2Fa+%2B+b", want: "acc-soft-delete%2Furl%2Fa+%2B+b"},
		{name: "invalid url encoding", key: "invalid%2", encodingType: "url", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := decodeSoftDeletedObjectKey(testCase.key, testCase.encodingType)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if got != testCase.want {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestUnitS3ObjectsSoftDeleteDataSource_FlattenSoftDeletedObjectContents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		objects []oosListObjectsSoftDeleteObj
		want    []map[string]any
	}{
		{
			name:    "empty",
			objects: []oosListObjectsSoftDeleteObj{},
			want:    []map[string]any{},
		},
		{
			name: "soft delete at",
			objects: []oosListObjectsSoftDeleteObj{{
				Key:          "current-key",
				SoftDeleteAt: "2026-08-14T12:00:00.000Z",
			}},
			want: []map[string]any{{
				"key":            "current-key",
				"soft_delete_at": "2026-08-14T12:00:00.000Z",
			}},
		},
		{
			name: "legacy soft delete at",
			objects: []oosListObjectsSoftDeleteObj{{
				Key:                "legacy-key",
				SoftDeleteAtLegacy: "2026-08-14T12:01:00.000Z",
			}},
			want: []map[string]any{{
				"key":            "legacy-key",
				"soft_delete_at": "2026-08-14T12:01:00.000Z",
			}},
		},
		{
			name: "mixed multiple objects",
			objects: []oosListObjectsSoftDeleteObj{
				{Key: "current-key", SoftDeleteAt: "2026-08-14T12:02:00.000Z"},
				{Key: "legacy-key", SoftDeleteAtLegacy: "2026-08-14T12:03:00.000Z"},
			},
			want: []map[string]any{
				{"key": "current-key", "soft_delete_at": "2026-08-14T12:02:00.000Z"},
				{"key": "legacy-key", "soft_delete_at": "2026-08-14T12:03:00.000Z"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := flattenSoftDeletedObjectContents(testCase.objects); !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("got %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestUnitS3ObjectsSoftDeleteDataSource_ListObjectsSoftDeleteQueryMarker(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		marker       string
		encodingType string
		want         string
	}{
		{
			name:   "plain marker without encoding type",
			marker: "1769925700000",
			want:   "1769925700000",
		},
		{
			name:         "url marker with special characters",
			marker:       "folder/special key%2Bvalue",
			encodingType: "url",
			want:         "folder%2Fspecial+key%2Bvalue",
		},
		{
			name:         "already encoded url marker",
			marker:       "folder%2Fspecial+key%2Bvalue",
			encodingType: "URL",
			want:         "folder%2Fspecial+key%2Bvalue",
		},
		{
			name:         "invalid url escape is encoded literally",
			marker:       "folder%2Gitem",
			encodingType: "url",
			want:         "folder%252Gitem",
		},
		{
			name:   "percent is escaped without url encoding type",
			marker: "folder%2Fitem",
			want:   "folder%252Fitem",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := listObjectsSoftDeleteQueryMarker(tc.marker, tc.encodingType); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnitS3ObjectsSoftDeleteDataSource_ListObjectsSoftDeleteResultXML(t *testing.T) {
	t.Parallel()

	var result oosListObjectsSoftDeleteResult
	if err := xml.Unmarshal([]byte(`
<ListObjectsSoftDeleteResult>
  <Name>test-bucket</Name>
  <SoftDeleteAfter>1769925600000</SoftDeleteAfter>
  <SoftDeleteBeforer>1769936400000</SoftDeleteBeforer>
  <Marker></Marker>
  <NextMarker>1769925700000/test3.txt</NextMarker>
  <MaxKeys>2</MaxKeys>
  <EncodingType>url</EncodingType>
  <IsTruncated>true</IsTruncated>
  <Contents>
    <Key>acc-soft-delete%2Furl%2Fa+%2B+b</Key>
    <LastModified>2012-09-01T01:56:20.000Z</LastModified>
    <SoftDeletAt>2026-02-01T12:16:40.000Z</SoftDeletAt>
    <ETag>bf1d737a4d46a19f3bced6905cc8b902</ETag>
    <Size>142863</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListObjectsSoftDeleteResult>`), &result); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if got, want := result.Name, "test-bucket"; got != want {
		t.Fatalf("got bucket name %q, want %q", got, want)
	}
	if got, want := result.softDeleteBefore(), "1769936400000"; got != want {
		t.Fatalf("got soft delete before %q, want %q", got, want)
	}
	if got, want := result.NextMarker, "1769925700000/test3.txt"; got != want {
		t.Fatalf("got next marker %q, want %q", got, want)
	}
	if got, want := result.IsTruncated, true; got != want {
		t.Fatalf("got is truncated %t, want %t", got, want)
	}
	if got, want := len(result.Contents), 1; got != want {
		t.Fatalf("got %d contents, want %d", got, want)
	}
	if got, want := result.Contents[0].softDeleteAt(), "2026-02-01T12:16:40.000Z"; got != want {
		t.Fatalf("got soft delete at %q, want %q", got, want)
	}
	if got, want := result.Contents[0].Key, "acc-soft-delete%2Furl%2Fa+%2B+b"; got != want {
		t.Fatalf("got encoded key %q, want %q", got, want)
	}
}
