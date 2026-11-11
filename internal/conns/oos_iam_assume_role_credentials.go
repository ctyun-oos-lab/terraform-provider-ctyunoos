// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	smithy "github.com/aws/smithy-go"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
)

type oosIAMAssumeRoleCredentialsProvider struct {
	credentials   aws.CredentialsProvider
	endpoint      string
	httpClient    *http.Client
	roles         []awsbase.AssumeRole
	signingRegion string
}

func newOOSIAMAssumeRoleCredentialsProvider(credentials aws.CredentialsProvider, endpoint string, httpClient *http.Client, roles []awsbase.AssumeRole, signingRegion string) aws.CredentialsProvider {
	return &oosIAMAssumeRoleCredentialsProvider{
		credentials:   credentials,
		endpoint:      endpoint,
		httpClient:    httpClient,
		roles:         roles,
		signingRegion: signingRegion,
	}
}

func (p *oosIAMAssumeRoleCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if p.credentials == nil {
		return aws.Credentials{}, fmt.Errorf("OOS IAM AssumeRole source credentials are not configured")
	}
	if p.endpoint == "" {
		return aws.Credentials{}, fmt.Errorf("OOS IAM AssumeRole endpoint is not configured")
	}
	if p.signingRegion == "" {
		return aws.Credentials{}, fmt.Errorf("OOS IAM AssumeRole signing region is not configured")
	}

	credentialsProvider := p.credentials
	var credentials aws.Credentials
	for _, role := range p.roles {
		var err error
		credentials, err = p.assumeRole(ctx, credentialsProvider, role)
		if err != nil {
			return aws.Credentials{}, err
		}

		credentialsProvider = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return credentials, nil
		})
	}

	return credentials, nil
}

func (p *oosIAMAssumeRoleCredentialsProvider) assumeRole(ctx context.Context, credentialsProvider aws.CredentialsProvider, role awsbase.AssumeRole) (aws.Credentials, error) {
	sourceCredentials, err := credentialsProvider.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("retrieving OOS IAM AssumeRole source credentials: %w", err)
	}

	body := url.Values{}
	body.Set("Action", "AssumeRole")
	body.Set("Version", "2010-05-08")
	body.Set("RoleArn", role.RoleARN)
	body.Set("RoleSessionName", role.SessionName)
	if role.Duration != 0 {
		body.Set("DurationSeconds", strconv.FormatInt(int64(role.Duration/time.Second), 10))
	}
	if role.ExternalID != "" {
		body.Set("ExternalId", role.ExternalID)
	}
	if role.Policy != "" {
		body.Set("Policy", role.Policy)
	}
	if role.SourceIdentity != "" {
		body.Set("SourceIdentity", role.SourceIdentity)
	}

	encodedBody := body.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, strings.NewReader(encodedBody))
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("creating OOS IAM AssumeRole request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("X-Amz-Content-Sha256", OOSIAMUnsignedPayload)
	req.ContentLength = int64(len(encodedBody))

	signer := OOSIAMHTTPSigner{
		SigningName:   OOSIAMSigningName,
		SigningRegion: p.signingRegion,
	}
	if err := signer.SignHTTP(ctx, sourceCredentials, req, OOSIAMUnsignedPayload, OOSIAMSigningName, p.signingRegion, time.Now().UTC()); err != nil {
		return aws.Credentials{}, fmt.Errorf("signing OOS IAM AssumeRole request: %w", err)
	}

	httpClient := p.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("calling OOS IAM AssumeRole: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("reading OOS IAM AssumeRole response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aws.Credentials{}, fmt.Errorf("calling OOS IAM AssumeRole: %w", parseOOSIAMAssumeRoleError(resp, responseBody))
	}

	var output oosIAMAssumeRoleResponse
	if err := xml.Unmarshal(responseBody, &output); err != nil {
		return aws.Credentials{}, fmt.Errorf("decoding OOS IAM AssumeRole response: %w", err)
	}

	creds := output.Result.Credentials
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" || creds.SessionToken == "" {
		return aws.Credentials{}, fmt.Errorf("decoding OOS IAM AssumeRole response: missing credentials")
	}

	expires, err := parseOOSIAMAssumeRoleExpiration(creds.Expiration)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("decoding OOS IAM AssumeRole response expiration: %w", err)
	}

	return aws.Credentials{
		AccessKeyID:     strings.TrimSpace(creds.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(creds.SecretAccessKey),
		SessionToken:    strings.TrimSpace(creds.SessionToken),
		Source:          "OOS IAM AssumeRole",
		CanExpire:       !expires.IsZero(),
		Expires:         expires,
	}, nil
}

func parseOOSIAMAssumeRoleError(resp *http.Response, responseBody []byte) error {
	var parsed struct {
		Code      string `xml:"Code"`
		Message   string `xml:"Message"`
		RequestID string `xml:"RequestId"`
	}

	responseText := strings.TrimSpace(string(responseBody))
	if err := xml.Unmarshal(responseBody, &parsed); err != nil {
		return fmt.Errorf("HTTP %d: unable to parse XML error response: %s", resp.StatusCode, responseText)
	}

	code := strings.TrimSpace(parsed.Code)
	if code == "" {
		code = "UnknownError"
	}

	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = responseText
	}
	if message == "" {
		message = code
	}

	requestID := strings.TrimSpace(parsed.RequestID)
	if requestID == "" {
		requestID = resp.Header.Get("X-Amz-Request-Id")
	}
	if requestID == "" {
		requestID = resp.Header.Get("X-Request-Id")
	}

	apiErr := &smithy.GenericAPIError{Code: code, Message: message}
	if requestID == "" {
		return fmt.Errorf("HTTP %d: %w", resp.StatusCode, apiErr)
	}

	return fmt.Errorf("HTTP %d, RequestID %s: %w", resp.StatusCode, requestID, apiErr)
}

func parseOOSIAMAssumeRoleExpiration(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, nil
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time format %q", v)
}

type oosIAMAssumeRoleResponse struct {
	Result oosIAMAssumeRoleResult `xml:"AssumeRoleResult"`
}

type oosIAMAssumeRoleResult struct {
	Credentials oosIAMAssumeRoleCredentials `xml:"Credentials"`
}

type oosIAMAssumeRoleCredentials struct {
	AccessKeyID     string `xml:"AccessKeyId"`
	SecretAccessKey string `xml:"SecretAccessKey"`
	SessionToken    string `xml:"SessionToken"`
	Expiration      string `xml:"Expiration"`
}
