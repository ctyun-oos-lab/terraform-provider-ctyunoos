// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	smithy "github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestNewOOSIAMInvalidHTTPResponseError(t *testing.T) {
	t.Parallel()

	response := &http.Response{
		StatusCode: 0,
		Header: http.Header{
			"Content-Type":                  {"text/html"},
			"Server":                        {"iam-gateway"},
			"Set-Cookie":                    {"must-not-be-reported"},
			"X-Envoy-Upstream-Service-Time": {"12"},
		},
		Request: &http.Request{URL: &url.URL{Host: "iam.example.test"}},
	}

	err := newOOSIAMInvalidHTTPResponseError(response, []byte("upstream unavailable"), nil)

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error does not implement smithy.APIError: %T", err)
	}
	if got, want := apiErr.ErrorCode(), oosIAMInvalidHTTPResponseCode; got != want {
		t.Errorf("ErrorCode() = %q, want %q", got, want)
	}

	message := apiErr.ErrorMessage()
	for _, expected := range []string{"iam.example.test", "Content-Type=\"text/html\"", "Server=\"iam-gateway\"", "X-Envoy-Upstream-Service-Time=\"12\"", "upstream unavailable"} {
		if !strings.Contains(message, expected) {
			t.Errorf("ErrorMessage() = %q, expected %q", message, expected)
		}
	}
	if strings.Contains(message, "must-not-be-reported") {
		t.Errorf("ErrorMessage() contains a sensitive response header: %q", message)
	}
}

func TestHandleOOSIAMErrorResponse_PreservesDeserializeError(t *testing.T) {
	t.Parallel()

	response := &smithyhttp.Response{Response: &http.Response{StatusCode: 0}}
	var metadata middleware.Metadata

	err := handleOOSIAMErrorResponse(response, &metadata, errors.New("unexpected EOF"))

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error does not implement smithy.APIError: %T", err)
	}
	if got, want := apiErr.ErrorCode(), oosIAMInvalidHTTPResponseCode; got != want {
		t.Errorf("ErrorCode() = %q, want %q", got, want)
	}
	if got := apiErr.ErrorMessage(); !strings.Contains(got, `cause="unexpected EOF"`) {
		t.Errorf("ErrorMessage() = %q, want deserialize error", got)
	}
}

func TestOOSIAMInvalidHTTPResponseBodyTruncation(t *testing.T) {
	t.Parallel()

	body := []byte(strings.Repeat("x", oosIAMDiagnosticBodyLimit+1))
	got := oosIAMTruncateDiagnosticBody(body)
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("oosIAMTruncateDiagnosticBody() = %q, want truncated suffix", got)
	}
}

func TestOOSIAMInvalidHTTPResponseRetryable(t *testing.T) {
	t.Parallel()

	err := &smithy.OperationError{
		ServiceID:     "IAM",
		OperationName: "CreateUser",
		Err:           newOOSIAMAPIError(0, oosIAMInvalidHTTPResponseCode, "invalid response"),
	}

	if !retry.AddWithErrorCodes(retry.NewStandard(), oosIAMInvalidHTTPResponseCode).IsErrorRetryable(err) {
		t.Errorf("InvalidHTTPResponse is not retryable")
	}
}
