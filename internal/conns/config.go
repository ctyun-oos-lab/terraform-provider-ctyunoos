// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/version"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
	basediag "github.com/hashicorp/aws-sdk-go-base/v2/diag"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
	"github.com/hashicorp/aws-sdk-go-base/v2/logging"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

type Config struct {
	AccessKey                      string
	AssumeRole                     []awsbase.AssumeRole
	CustomCABundle                 string
	Endpoints                      map[string]string
	HTTPProxy                      *string
	HTTPSProxy                     *string
	Insecure                       bool
	MaxRetries                     int
	NoProxy                        string
	RetryMode                      aws.RetryMode
	S3UsePathStyle                 bool
	SecretKey                      string
	SuppressDebugLog               bool
	TerraformVersion               string
	Token                          string
	TokenBucketRateLimiterCapacity int
	UserAgent                      awsbase.UserAgentProducts
}

// ConfigureProvider configures the provided provider Meta (instance data).
func (c *Config) ConfigureProvider(ctx context.Context, client *AWSClient) (*AWSClient, diag.Diagnostics) {
	var diags diag.Diagnostics

	ctx, logger := logging.NewTfLogger(ctx)

	// OOS configuration contract:
	//   - An endpoint for each supported service is mandatory, and each endpoint
	//     must match the service's template so a region can be reverse-parsed
	//     from it (that region is used for SigV4 signing).
	requiredServices := []struct {
		name string
		attr string
	}{
		{names.S3, "endpoints.s3"},
		{names.IAM, "endpoints.iam"},
		{names.CloudTrail, "endpoints.cloudtrail"},
	}

	// OOS allows configuring endpoints for only a subset of services, but requires at least one, so that a signing region can be reverse-derived.
	hasAnyEndpoint := false
	for _, svc := range requiredServices {
		if c.Endpoints[svc.name] != "" {
			hasAnyEndpoint = true
			break
		}
	}
	if !hasAnyEndpoint {
		diags = append(diags, errs.NewErrorDiagnostic(
			"Missing required provider configuration",
			"At least one of endpoints.s3, endpoints.iam, or endpoints.cloudtrail must be configured in the provider block.",
		))
	}

	// Continue with format validation for the configured endpoints.
	for _, svc := range requiredServices {
		endpoint := c.Endpoints[svc.name]
		if endpoint == "" {
			continue
		}
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			diags = append(diags, errs.NewErrorDiagnostic(
				"Invalid provider configuration",
				fmt.Sprintf("The %s endpoint %q must be an absolute URL with an http or https scheme and a host.", svc.name, endpoint),
			))
			continue
		}
		if names.RegionForEndpoint(svc.name, endpoint) == "" {
			diags = append(diags, errs.NewErrorDiagnostic(
				"Invalid provider configuration",
				fmt.Sprintf("The %s endpoint %q does not match the expected OOS endpoint format, so its region cannot be determined. Configure %s with an endpoint of the documented form.", svc.name, endpoint, svc.attr),
			))
		}
	}
	if diags.HasError() {
		return nil, diags
	}

	if c.AccessKey == "" || c.SecretKey == "" {
		diags = append(diags, errs.NewErrorDiagnostic(
			"Missing OOS Credentials",
			"Configure both access_key and secret_key, or set both CTYUNOOS_ACCESS_KEY and CTYUNOOS_SECRET_KEY. AWS profiles, shared credential files, and the default AWS credential chain are not supported.",
		))
		return nil, diags
	}

	const (
		maxBackoff = 300 * time.Second // AWS SDK for Go v1 DefaultRetryerMaxRetryDelay: https://github.com/aws/aws-sdk-go/blob/9f6e3bb9f523aef97fa1cd5c5f8ba8ecf212e44e/aws/client/default_retryer.go#L48-L49.
	)

	assumeRoles := c.AssumeRole

	awsbaseConfig := awsbase.Config{
		AccessKey:     c.AccessKey,
		Backoff:       &v1CompatibleBackoff{maxRetryDelay: maxBackoff},
		CallerName:    "Terraform CTYUN OOS Provider",
		IamEndpoint:   c.Endpoints[names.IAM],
		Insecure:      c.Insecure,
		HTTPClient:    client.HTTPClient(ctx),
		HTTPProxy:     c.HTTPProxy,
		HTTPSProxy:    c.HTTPSProxy,
		HTTPProxyMode: awsbase.HTTPProxyModeLegacy,
		Logger:        logger,
		MaxBackoff:    maxBackoff,
		MaxRetries:    c.MaxRetries,
		NoProxy:       c.NoProxy,
		// OOS derives the real region from each service endpoint. The SDK requires
		// a syntactically valid region only for client initialization.
		Region:    "us-east-1", // lintignore:AWSAT003
		RetryMode: c.RetryMode,
		SecretKey: c.SecretKey,
		// OOS does not expose the AWS STS credential-validation API.
		SkipCredsValidation:            true,
		SkipRequestingAccountId:        true,
		SuppressDebugLog:               c.SuppressDebugLog,
		Token:                          c.Token,
		TokenBucketRateLimiterCapacity: c.TokenBucketRateLimiterCapacity,
		UserAgent:                      oosUserAgent(c.UserAgent),
	}

	if c.CustomCABundle != "" {
		awsbaseConfig.CustomCABundle = c.CustomCABundle
	}

	tflog.Debug(ctx, "Configuring Terraform AWS Provider")
	ctx, cfg, awsDiags := awsbase.GetAwsConfig(ctx, &awsbaseConfig)

	for _, d := range awsDiags {
		diags = append(diags, diag.Diagnostic{
			Severity: baseSeverityToSDKSeverity(d.Severity()),
			Summary:  d.Summary(),
			Detail:   d.Detail(),
		})
	}

	if diags.HasError() {
		return nil, diags
	}

	// OOS derives the real region per service by reverse-parsing the configured
	// endpoint and injects it into SigV4 with signing middleware.

	if len(assumeRoles) > 0 {
		iamEndpoint := c.Endpoints[names.IAM]
		signingRegion := names.RegionForEndpoint(names.IAM, iamEndpoint)
		credentials := aws.NewCredentialsCache(newOOSIAMAssumeRoleCredentialsProvider(cfg.Credentials, iamEndpoint, client.HTTPClient(ctx), assumeRoles, signingRegion))
		if _, err := credentials.Retrieve(ctx); err != nil {
			return nil, sdkdiag.AppendErrorf(diags, "Cannot assume IAM Role: %s", err)
		}
		cfg.Credentials = credentials
	}

	tflog.Debug(ctx, "Retrieving AWS account details")
	accountID, partitionID, awsDiags := awsbase.GetAwsAccountIDAndPartition(ctx, cfg, &awsbaseConfig)
	for _, d := range awsDiags {
		diags = append(diags, diag.Diagnostic{
			Severity: baseSeverityToSDKSeverity(d.Severity()),
			Summary:  fmt.Sprintf("Retrieving AWS account details: %s", d.Summary()),
			Detail:   d.Detail(),
		})
	}

	err := awsbaseConfig.VerifyAccountIDAllowed(accountID)
	if err != nil {
		return nil, sdkdiag.AppendErrorf(diags, "%s", err.Error())
	}

	for _, partition := range endpoints.DefaultPartitions() {
		if partition.ID() == partitionID {
			client.partition = partition
		}
	}

	client.accountID = accountID
	client.terraformVersion = c.TerraformVersion

	// Used for lazy-loading AWS API clients.
	client.awsConfig = &cfg
	client.clients = make(map[string]map[string]any, 0)
	client.endpoints = c.Endpoints
	client.logger = logger
	client.s3UsePathStyle = c.S3UsePathStyle

	return client, diags
}

// oosUserAgent returns the provider-level products sent in User-Agent.
// A configured value replaces the OOS provider default rather than being
// appended to it. Module-level provider_meta products are appended later by
// the SDK base context middleware.
func oosUserAgent(userAgent awsbase.UserAgentProducts) awsbase.UserAgentProducts {
	if userAgent.BuildUserAgentString() != "" {
		return userAgent
	}

	return awsbase.UserAgentProducts{
		{Name: "terraform-provider-ctyunoos", Version: version.ProviderVersion},
	}
}

func baseSeverityToSDKSeverity(s basediag.Severity) diag.Severity {
	switch s {
	case basediag.SeverityWarning:
		return diag.Warning
	case basediag.SeverityError:
		return diag.Error
	default:
		return -1
	}
}
