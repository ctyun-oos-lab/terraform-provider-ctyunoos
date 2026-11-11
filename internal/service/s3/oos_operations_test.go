// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestUnitS3OOSOperations_WithContentMD5(t *testing.T) {
	t.Parallel()

	stack := middleware.NewStack("DeleteObjects", nil)
	// Test setup with fixed unique middleware IDs; Add cannot fail here.
	_ = stack.Initialize.Add(middleware.InitializeMiddlewareFunc("AWSChecksum:SetupInputContext", func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
		return next.HandleInitialize(ctx, in)
	}), middleware.Before)
	_ = stack.Build.Add(middleware.BuildMiddlewareFunc("AWSChecksum:RequestMetricsTracking", func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
		return next.HandleBuild(ctx, in)
	}), middleware.Before)
	_ = stack.Build.Add(middleware.BuildMiddlewareFunc("ContentChecksum", func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
		return next.HandleBuild(ctx, in)
	}), middleware.Before)
	_ = stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("AWSChecksum:ComputeInputPayloadChecksum", func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
		return next.HandleFinalize(ctx, in)
	}), middleware.Before)
	_ = stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("addInputChecksumTrailer", func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
		return next.HandleFinalize(ctx, in)
	}), middleware.Before)

	var options s3.Options
	withContentMD5(&options)
	for _, fn := range options.APIOptions {
		if err := fn(stack); err != nil {
			t.Fatalf("applying API option: %s", err)
		}
	}

	if got, ok := stack.Initialize.Get("AWSChecksum:SetupInputContext"); ok || got != nil {
		t.Fatalf("SetupInputContext middleware still present")
	}
	if got, ok := stack.Build.Get("AWSChecksum:RequestMetricsTracking"); ok || got != nil {
		t.Fatalf("RequestMetricsTracking middleware still present")
	}
	if got, ok := stack.Finalize.Get("AWSChecksum:ComputeInputPayloadChecksum"); ok || got != nil {
		t.Fatalf("ComputeInputPayloadChecksum middleware still present")
	}
	if got, ok := stack.Finalize.Get("addInputChecksumTrailer"); ok || got != nil {
		t.Fatalf("addInputChecksumTrailer middleware still present")
	}

	contentChecksum, ok := stack.Build.Get("ContentChecksum")
	if !ok || contentChecksum == nil {
		t.Fatalf("ContentChecksum middleware missing")
	}
	if got := contentChecksum.ID(); got != "ContentChecksum" {
		t.Fatalf("got ContentChecksum middleware ID %q, want %q", got, "ContentChecksum")
	}
}

func TestUnitS3OOSOperations_SetRequestMethodMiddleware(t *testing.T) {
	t.Parallel()

	stack := middleware.NewStack("RecoverObjects", nil)
	if err := setRequestMethodMiddleware("DELETE")(stack); err != nil {
		t.Fatalf("adding middleware: %s", err)
	}

	methodMiddleware, ok := stack.Build.Get("OOSSetRequestMethod")
	if !ok || methodMiddleware == nil {
		t.Fatalf("OOSSetRequestMethod middleware missing")
	}
	if got := methodMiddleware.ID(); got != "OOSSetRequestMethod" {
		t.Fatalf("got middleware ID %q, want %q", got, "OOSSetRequestMethod")
	}
}

func TestUnitS3OOSOperations_RecoverObjectsRawQuery(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input oosRecoverObjectsInput
		want  string
	}{
		{
			name: "after and before",
			input: oosRecoverObjectsInput{
				softDeleteAfter:  "1769158000000",
				softDeleteBefore: "1769750800000",
			},
			want: "recover&soft-delete-after=1769158000000&soft-delete-before=1769750800000",
		},
		{
			name: "after only",
			input: oosRecoverObjectsInput{
				softDeleteAfter: "1769158000000",
			},
			want: "recover&soft-delete-after=1769158000000",
		},
		{
			name: "before only",
			input: oosRecoverObjectsInput{
				softDeleteBefore: "1769750800000",
			},
			want: "recover&soft-delete-before=1769750800000",
		},

		{
			name: "url encoding type",
			input: oosRecoverObjectsInput{
				encodingType: "url",
			},
			want: "recover&encoding-type=url",
		},
		{
			name:  "no window",
			input: oosRecoverObjectsInput{},
			want:  "recover",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := recoverObjectsRawQuery(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnitS3OOSOperations_ListObjectsSoftDeleteRawQuery(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input oosListObjectsSoftDeleteInput
		want  string
	}{
		{
			name: "full query",
			input: oosListObjectsSoftDeleteInput{
				softDeleteAfter:  "1769925600000",
				softDeleteBefore: "1769936400000",
				maxKeys:          2,
				marker:           "1769925700000/test3.txt",
				encodingType:     "url",
			},
			want: "soft-delete-objects&soft-delete-after=1769925600000&soft-delete-before=1769936400000&max-keys=2&marker=1769925700000%2Ftest3.txt&encoding-type=url",
		},
		{
			name: "encoded marker stays stable for url encoding",
			input: oosListObjectsSoftDeleteInput{
				marker:       "1769925700000%2Ftest3.txt",
				encodingType: "url",
			},
			want: "soft-delete-objects&marker=1769925700000%2Ftest3.txt&encoding-type=url",
		},
		{
			name:  "no optional fields",
			input: oosListObjectsSoftDeleteInput{},
			want:  "soft-delete-objects",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := listObjectsSoftDeleteRawQuery(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnitS3OOSOperations_MarshalRecoverObjectsBody(t *testing.T) {
	t.Parallel()

	body, err := marshalRecoverObjectsBody([]string{"test1.txt", "dir/test2.txt"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	got := string(body)
	for _, want := range []string{
		`<Recover xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`,
		`<Object><Key>test1.txt</Key></Object>`,
		`<Object><Key>dir/test2.txt</Key></Object>`,
		`</Recover>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("body %q does not contain %q", got, want)
		}
	}
}

func TestUnitS3OOSOperations_MarshalRecoverObjectsBodyURL(t *testing.T) {
	t.Parallel()

	body, err := marshalRecoverObjectsBody([]string{"dir/special key+value"}, "url")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if got, want := string(body), `<Object><Key>dir%2Fspecial+key%2Bvalue</Key></Object>`; !strings.Contains(got, want) {
		t.Fatalf("body %q does not contain %q", got, want)
	}
}

func TestUnitS3OOSOperations_ParseOOSError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		statusCode    int
		data          string
		wantCode      string
		wantMessage   string
		wantErrorText string
	}{
		{
			name:        "XML error response",
			statusCode:  404,
			data:        `<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`,
			wantCode:    "NoSuchKey",
			wantMessage: "The specified key does not exist.",
		},
		{
			name:          "empty body",
			statusCode:    500,
			wantErrorText: "OOS API error (HTTP 500): ",
		},
		{
			name:          "non XML body",
			statusCode:    502,
			data:          "upstream unavailable",
			wantErrorText: "OOS API error (HTTP 502): upstream unavailable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := parseOOSError(tc.statusCode, []byte(tc.data))
			if tc.wantCode != "" {
				var apiErr smithy.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("got error type %T, want smithy.APIError", err)
				}
				if got := apiErr.ErrorCode(); got != tc.wantCode {
					t.Fatalf("got error code %q, want %q", got, tc.wantCode)
				}
				if got := apiErr.ErrorMessage(); got != tc.wantMessage {
					t.Fatalf("got error message %q, want %q", got, tc.wantMessage)
				}
				return
			}

			if got := err.Error(); got != tc.wantErrorText {
				t.Fatalf("got error %q, want %q", got, tc.wantErrorText)
			}
		})
	}
}
func TestUnitS3OOSOperations_ParseRecoverObjectsResult(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		payload          string
		wantRecoveredLen int
		wantErrorLen     int
		wantFirstKey     string
		wantFirstErrKey  string
	}{
		{
			name:             "empty body",
			payload:          "",
			wantRecoveredLen: 0,
			wantErrorLen:     0,
		},
		{
			name:             "empty xml result",
			payload:          `<RecoveredResult/>`,
			wantRecoveredLen: 0,
			wantErrorLen:     0,
		},
		{
			name: "xml mixed result",
			payload: `
<RecoveredResult>
  <Recovered>
    <Key>key1</Key>
  </Recovered>
  <Error>
    <Key>key2</Key>
    <Code>AccessDenied</Code>
    <Message>AccessDenied</Message>
  </Error>
</RecoveredResult>`,
			wantRecoveredLen: 1,
			wantErrorLen:     1,
			wantFirstKey:     "key1",
			wantFirstErrKey:  "key2",
		},
		{
			name: "xml recovered only",
			payload: `
<RecoveredResult>
  <Recovered>
    <Key>key1</Key>
  </Recovered>
</RecoveredResult>`,
			wantRecoveredLen: 1,
			wantErrorLen:     0,
			wantFirstKey:     "key1",
		},
		{
			name: "xml errors only",
			payload: `
<RecoveredResult>
  <Error>
    <Key>key2</Key>
    <Code>NoSuchKey</Code>
    <Message>Not in soft delete retention</Message>
  </Error>
</RecoveredResult>`,
			wantRecoveredLen: 0,
			wantErrorLen:     1,
			wantFirstErrKey:  "key2",
		},
		{
			name:             "json empty result",
			payload:          `{}`,
			wantRecoveredLen: 0,
			wantErrorLen:     0,
		},
		{
			name: "json mixed result",
			payload: `{
				"Recovered": [{"Key": "key1"}],
				"Error": [{"Key": "key2", "Code": "AccessDenied", "Message": "AccessDenied"}]
			}`,
			wantRecoveredLen: 1,
			wantErrorLen:     1,
			wantFirstKey:     "key1",
			wantFirstErrKey:  "key2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseRecoverObjectsResult([]byte(tc.payload))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if got := len(result.Recovered); got != tc.wantRecoveredLen {
				t.Fatalf("got %d recovered objects, want %d", got, tc.wantRecoveredLen)
			}
			if got := len(result.RecoverError); got != tc.wantErrorLen {
				t.Fatalf("got %d recover errors, want %d", got, tc.wantErrorLen)
			}
			if tc.wantFirstKey != "" {
				if got := result.Recovered[0].Key; got != tc.wantFirstKey {
					t.Fatalf("got recovered key %q, want %q", got, tc.wantFirstKey)
				}
			}
			if tc.wantFirstErrKey != "" {
				if got := result.RecoverError[0].Key; got != tc.wantFirstErrKey {
					t.Fatalf("got error key %q, want %q", got, tc.wantFirstErrKey)
				}
			}
		})
	}
}

func TestUnitS3ObjectDataSource_CaptureOOSObjectHeadersMiddleware(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		headers http.Header
		want    oosObjectHeaders
	}{
		"all headers": {
			headers: http.Header{
				oosHeaderContentMD5:       {"1B2M2Y8AsgTpgAmY7PhCfg=="},
				oosHeaderDataLocation:     {"ChengDu"},
				oosHeaderMetadataLocation: {"WuHan"},
			},
			want: oosObjectHeaders{
				contentMD5:       "1B2M2Y8AsgTpgAmY7PhCfg==",
				dataLocation:     "ChengDu",
				metadataLocation: "WuHan",
			},
		},
		"missing headers": {},
		"only content MD5": {
			headers: http.Header{
				oosHeaderContentMD5: {"XUFAKrxLKna5cZ2REBfFkg=="},
			},
			want: oosObjectHeaders{contentMD5: "XUFAKrxLKna5cZ2REBfFkg=="},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var got oosObjectHeaders
			headers := make(http.Header)
			for key, values := range tc.headers {
				for _, value := range values {
					headers.Add(key, value)
				}
			}

			stack := middleware.NewStack("HeadObject", nil)
			if err := captureOOSObjectHeadersMiddleware(&got)(stack); err != nil {
				t.Fatalf("adding middleware: %s", err)
			}

			capturer, ok := stack.Deserialize.Get("OOSCaptureObjectHeaders")
			if !ok {
				t.Fatal("OOSCaptureObjectHeaders middleware missing")
			}

			_, _, err := capturer.HandleDeserialize(
				context.Background(),
				middleware.DeserializeInput{},
				middleware.DeserializeHandlerFunc(func(context.Context, middleware.DeserializeInput) (middleware.DeserializeOutput, middleware.Metadata, error) {
					return middleware.DeserializeOutput{
						RawResponse: &smithyhttp.Response{Response: &http.Response{Header: headers}},
					}, middleware.Metadata{}, nil
				}),
			)
			if err != nil {
				t.Fatalf("handling response: %s", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("captured headers = %#v, want %#v", got, tc.want)
			}
		})
	}
}
