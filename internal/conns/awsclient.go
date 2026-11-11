// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"math/rand" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used -- Deterministic PRNG required for VCR test reproducibility
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	apigatewayv2_types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/dns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
	baselogging "github.com/hashicorp/aws-sdk-go-base/v2/logging"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type AWSClient struct {
	accountID        string
	awsConfig        *aws.Config
	clients          map[string]map[string]any // Region -> service package name -> API client.
	endpoints        map[string]string         // From provider configuration.
	httpClient       *http.Client
	lock             sync.Mutex
	logger           baselogging.Logger
	partition        endpoints.Partition
	randomnessSource rand.Source // For VCR deterministic randomness.
	servicePackages  map[string]ServicePackage
	s3ExpressClient  *s3.Client
	s3UsePathStyle   bool   // From provider configuration.
	terraformVersion string // From provider configuration.
}

func (c *AWSClient) SetServicePackages(_ context.Context, servicePackages map[string]ServicePackage) {
	c.servicePackages = maps.Clone(servicePackages)
}

func (c *AWSClient) ServicePackage(_ context.Context, name string) ServicePackage {
	sp, ok := c.servicePackages[name]
	if !ok {
		return nil
	}
	return sp
}

func (c *AWSClient) ServicePackages(_ context.Context) iter.Seq[ServicePackage] {
	return maps.Values(c.servicePackages)
}

func (c *AWSClient) TerraformVersion(_ context.Context) string {
	return c.terraformVersion
}

// CredentialsProvider returns the AWS SDK for Go v2 credentials provider.
func (c *AWSClient) CredentialsProvider(context.Context) aws.CredentialsProvider {
	if c.awsConfig == nil {
		return nil
	}
	return c.awsConfig.Credentials
}

func (c *AWSClient) DefaultTagsConfig(context.Context) *tftags.DefaultConfig {
	return nil
}

func (c *AWSClient) IgnoreTagsConfig(context.Context) *tftags.IgnoreConfig {
	return nil
}

func (c *AWSClient) TagPolicyConfig(context.Context) *tftags.TagPolicyConfig {
	return nil
}

func (c *AWSClient) AwsConfig(context.Context) aws.Config { // nosemgrep:ci.aws-in-func-name
	return c.awsConfig.Copy()
}

// AccountID returns the configured AWS account ID.
func (c *AWSClient) AccountID(context.Context) string {
	return c.accountID
}

// Partition returns the ID of the configured AWS partition.
func (c *AWSClient) Partition(context.Context) string {
	return c.partition.ID()
}

// RandomnessSource returns the VCR randomness source if set.
func (c *AWSClient) RandomnessSource() rand.Source {
	return c.randomnessSource
}

// SetRandomnessSource sets the VCR randomness source.
//
// This should only be called during provider configuration for VCR testing.
func (c *AWSClient) SetRandomnessSource(source rand.Source) {
	c.randomnessSource = source
}

// Region returns the ID of the effective AWS Region.
// If the currently in-process operation has defined a per-resource Region override,
// that value is returned, otherwise the configured Region is returned.
func (c *AWSClient) Region(ctx context.Context) string {
	if inContext, ok := FromContext(ctx); ok {
		if r := inContext.OverrideRegion(); r != "" {
			return r
		}
	}

	return c.awsConfig.Region
}

// PartitionHostname returns a hostname with the provider domain suffix for the partition
// e.g. PREFIX.amazonaws.com
// The prefix should not contain a trailing period.
func (c *AWSClient) PartitionHostname(ctx context.Context, prefix string) string {
	return fmt.Sprintf("%s.%s", prefix, c.DNSSuffix(ctx))
}

// GlobalARN returns a global (no Region) ARN for the specified service namespace and resource.
func (c *AWSClient) GlobalARN(ctx context.Context, service, resource string) string {
	return c.arn(ctx, service, "", c.AccountID(ctx), resource)
}

// GlobalARNNoAccount returns a global (no Region) ARN for the specified service namespace and resource without AWS account ID.
func (c *AWSClient) GlobalARNNoAccount(ctx context.Context, service, resource string) string {
	return c.arn(ctx, service, "", "", resource)
}

// GlobalARNWithAccount returns a global (no Region) ARN for the specified service namespace, resource and account ID.
func (c *AWSClient) GlobalARNWithAccount(ctx context.Context, service, accountID, resource string) string {
	return c.arn(ctx, service, "", accountID, resource)
}

// RegionalARN returns a regional ARN for the specified service namespace and resource.
func (c *AWSClient) RegionalARN(ctx context.Context, service, resource string) string {
	return c.arn(ctx, service, c.Region(ctx), c.AccountID(ctx), resource)
}

// RegionalARNNoAccount returns a regional ARN for the specified service namespace and resource without AWS account ID.
func (c *AWSClient) RegionalARNNoAccount(ctx context.Context, service, resource string) string {
	return c.arn(ctx, service, c.Region(ctx), "", resource)
}

// RegionalARNWithAccount returns a regional ARN for the specified service namespace, resource and account ID.
func (c *AWSClient) RegionalARNWithAccount(ctx context.Context, service, accountID, resource string) string {
	return c.arn(ctx, service, c.Region(ctx), accountID, resource)
}

// RegionalARNWithRegion returns a regional ARN for the specified service namespace, resource and account ID.
func (c *AWSClient) RegionalARNWithRegion(ctx context.Context, service, region, resource string) string {
	return c.arn(ctx, service, region, c.AccountID(ctx), resource)
}

// arn returns an ARN for the specified service namespace, region, account ID and resource.
func (c *AWSClient) arn(ctx context.Context, service, region, accountID, resource string) string {
	return arn.ARN{
		Partition: c.Partition(ctx),
		Service:   service,
		Region:    region,
		AccountID: accountID,
		Resource:  resource,
	}.String()
}

// RegionalHostname returns a hostname with the provider domain suffix for the region and partition
// e.g. PREFIX.us-west-2.amazonaws.com
// The prefix should not contain a trailing period.
func (c *AWSClient) RegionalHostname(ctx context.Context, prefix string) string {
	return fmt.Sprintf("%s.%s.%s", prefix, c.Region(ctx), c.DNSSuffix(ctx))
}

// S3ExpressClient returns an AWS SDK for Go v2 S3 API client suitable for use with S3 Express (directory buckets).
// This client differs from the standard S3 API client only in us-east-1 if the global S3 endpoint is used.
// In that case the returned client uses the regional S3 endpoint.
func (c *AWSClient) S3ExpressClient(ctx context.Context) *s3.Client {
	s3Client := c.S3Client(ctx)

	c.lock.Lock() // OK since a non-default client is created.
	defer c.lock.Unlock()

	if c.s3ExpressClient == nil {
		if s3Client.Options().Region == endpoints.AwsGlobalRegionID {
			// No global endpoint for S3 Express.
			// A non-empty extra map bypasses the default-client cache while c.lock is held.
			c.s3ExpressClient = errs.Must(client[*s3.Client](ctx, c, names.S3, map[string]any{"s3_express": true}))
		} else {
			c.s3ExpressClient = s3Client
		}
	}

	return c.s3ExpressClient
}

// S3UsePathStyle returns the s3_force_path_style provider configuration value.
func (c *AWSClient) S3UsePathStyle(context.Context) bool {
	return c.s3UsePathStyle
}

// effectiveRealRegionForService computes the real OOS region in effect for the
// given service. OOS does not configure a region on the provider; the region is
// derived by reverse-parsing the service's configured endpoint against its
// template. For S3 only, a per-resource override takes precedence. Unlike
// Region(), this never returns the SDK-dummy region, so it is the value used for
// endpoint host generation, SigV4 signing and state write-back.
//
// IAM and CloudTrail do not expose a region attribute, so they always resolve to
// the region of their provider-configured endpoint.
func (c *AWSClient) effectiveRealRegionForService(ctx context.Context, servicePackageName string) string {
	if inContext, ok := FromContext(ctx); ok {
		if r := inContext.OverrideRegion(); r != "" {
			return r
		}
	}
	return names.RegionForEndpoint(servicePackageName, c.endpoints[servicePackageName])
}

// EffectiveRealRegion returns the real OOS region in effect for the current
// operation, resolving the service from the request context. See
// effectiveRealRegionForService for the resolution order.
func (c *AWSClient) EffectiveRealRegion(ctx context.Context) string {
	var servicePackageName string
	if inContext, ok := FromContext(ctx); ok {
		servicePackageName = inContext.ServicePackageName()
	}
	return c.effectiveRealRegionForService(ctx, servicePackageName)
}

// SetHTTPClient sets the http.Client used for AWS API calls.
func (c *AWSClient) SetHTTPClient(_ context.Context, httpClient *http.Client) {
	c.httpClient = httpClient
}

// HTTPClient returns the http.Client used for AWS API calls.
func (c *AWSClient) HTTPClient(context.Context) *http.Client {
	return c.httpClient
}

// RegisterLogger places the configured logger into Context so it can be used via `tflog`.
func (c *AWSClient) RegisterLogger(ctx context.Context) context.Context {
	return baselogging.RegisterLogger(ctx, c.logger)
}

// APIGatewayInvokeURL returns the Amazon API Gateway (REST APIs) invoke URL for the configured AWS Region.
// See https://docs.aws.amazon.com/apigateway/latest/developerguide/how-to-call-api.html.
func (c *AWSClient) APIGatewayInvokeURL(ctx context.Context, restAPIID, stageName string) string {
	return fmt.Sprintf("https://%s/%s", c.RegionalHostname(ctx, fmt.Sprintf("%s.execute-api", restAPIID)), stageName)
}

// APIGatewayV2InvokeURL returns the Amazon API Gateway v2 (WebSocket & HTTP APIs) invoke URL for the configured AWS Region.
// See https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-publish.html and
// https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-set-up-websocket-deployment.html.
func (c *AWSClient) APIGatewayV2InvokeURL(ctx context.Context, protocolType apigatewayv2_types.ProtocolType, apiID, stageName string) string {
	if protocolType == apigatewayv2_types.ProtocolTypeWebsocket {
		return fmt.Sprintf("wss://%s/%s", c.RegionalHostname(ctx, fmt.Sprintf("%s.execute-api", apiID)), stageName)
	}

	if stageName == "$default" {
		return fmt.Sprintf("https://%s/", c.RegionalHostname(ctx, fmt.Sprintf("%s.execute-api", apiID)))
	}

	return fmt.Sprintf("https://%s/%s", c.RegionalHostname(ctx, fmt.Sprintf("%s.execute-api", apiID)), stageName)
}

// CloudFrontDistributionHostedZoneID returns the Route 53 hosted zone ID
// for Amazon CloudFront distributions in the configured AWS partition.
func (c *AWSClient) CloudFrontDistributionHostedZoneID(ctx context.Context) string {
	if c.Partition(ctx) == endpoints.AwsCnPartitionID {
		return "Z3RFFRIM2A3IF5" // See https://docs.amazonaws.cn/en_us/aws/latest/userguide/route53.html
	}
	return "Z2FDTNDATAQYW2" // See https://docs.aws.amazon.com/Route53/latest/APIReference/API_AliasTarget.html#Route53-Type-AliasTarget-HostedZoneId
}

// DefaultKMSKeyPolicy returns the default policy for KMS keys in the configured AWS partition.
func (c *AWSClient) DefaultKMSKeyPolicy(ctx context.Context) string {
	return fmt.Sprintf(`
{
	"Id": "default",
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "Enable IAM User Permissions",
			"Effect": "Allow",
			"Principal": {
				"AWS": "arn:%[1]s:iam::%[2]s:root"
			},
			"Action": "kms:*",
			"Resource": "*"
		}
	]
}	
`, c.Partition(ctx), c.AccountID(ctx))
}

// GlobalAcceleratorHostedZoneID returns the Route 53 hosted zone ID
// for AWS Global Accelerator accelerators in the configured AWS partition.
func (c *AWSClient) GlobalAcceleratorHostedZoneID(context.Context) string {
	return "Z2BJ6XQ5FK7U4H" // See https://docs.aws.amazon.com/general/latest/gr/global_accelerator.html#global_accelerator_region
}

// DNSSuffix returns the domain suffix for the configured AWS partition.
func (c *AWSClient) DNSSuffix(context.Context) string {
	dnsSuffix := c.partition.DNSSuffix()
	if dnsSuffix == "" {
		dnsSuffix = "amazonaws.com"
	}

	return dnsSuffix
}

// ReverseDNSPrefix returns the reverse DNS prefix for the configured AWS partition.
func (c *AWSClient) ReverseDNSPrefix(ctx context.Context) string {
	return dns.Reverse(c.DNSSuffix(ctx))
}

// EC2RegionalPrivateDNSSuffix returns the EC2 private DNS suffix for the configured AWS Region.
func (c *AWSClient) EC2RegionalPrivateDNSSuffix(ctx context.Context) string {
	region := c.Region(ctx)
	if region == endpoints.UsEast1RegionID {
		return "ec2.internal"
	}

	return fmt.Sprintf("%s.compute.internal", region)
}

// EC2RegionalPublicDNSSuffix returns the EC2 public DNS suffix for the configured AWS Region.
func (c *AWSClient) EC2RegionalPublicDNSSuffix(ctx context.Context) string {
	region := c.Region(ctx)
	if region == endpoints.UsEast1RegionID {
		return "compute-1"
	}

	return fmt.Sprintf("%s.compute", region)
}

// EC2PrivateDNSNameForIP returns a EC2 private DNS name in the configured AWS Region.
func (c *AWSClient) EC2PrivateDNSNameForIP(ctx context.Context, ip string) string {
	return fmt.Sprintf("ip-%s.%s", convertIPToDashIP(ip), c.EC2RegionalPrivateDNSSuffix(ctx))
}

// EC2PublicDNSNameForIP returns a EC2 public DNS name in the configured AWS Region.
func (c *AWSClient) EC2PublicDNSNameForIP(ctx context.Context, ip string) string {
	return c.PartitionHostname(ctx, fmt.Sprintf("ec2-%s.%s", convertIPToDashIP(ip), c.EC2RegionalPublicDNSSuffix(ctx)))
}

// ValidateInContextRegionInPartition verifies that the value of the top-level `region` attribute is in the configured AWS partition.
func (c *AWSClient) ValidateInContextRegionInPartition(ctx context.Context) error {
	if inContext, ok := FromContext(ctx); ok {
		if r, p := inContext.OverrideRegion(), c.Partition(ctx); r != "" && p != "" {
			if got, want := names.PartitionForRegion(r).ID(), p; got != want {
				return fmt.Errorf("partition (%s) for per-resource Region (%s) is not the provider's configured partition (%s)", got, r, want)
			}
		}
	}

	return nil
}

func convertIPToDashIP(ip string) string {
	return strings.Replace(ip, ".", "-", -1)
}

// apiClientConfig returns the AWS API client configuration parameters for the specified service.
func (c *AWSClient) apiClientConfig(ctx context.Context, servicePackageName string) map[string]any {
	// effectiveRealRegion is the OOS region in effect for this service: a
	// per-resource override for S3, otherwise the region reverse-parsed from the
	// service's configured endpoint. It drives both the endpoint host and the
	// SigV4 signing region.
	effectiveRealRegion := c.effectiveRealRegionForService(ctx, servicePackageName)

	// Endpoint resolution. The provider-configured endpoint is the default for
	// every service. For S3 only, a per-resource region override that targets a
	// DIFFERENT region than the configured endpoint rebases the configured
	// endpoint onto that region (replacing only the region segment of the host)
	// so the request reaches that region's host; IAM and CloudTrail always use
	// the provider-configured endpoint.
	//
	// When the override resolves to the same region as the configured endpoint
	// (the common case, since the region attribute defaults to the endpoint's
	// reverse-parsed region), we keep the configured endpoint verbatim.
	//
	// Rebasing (vs. regenerating from the bare template) preserves the configured
	// endpoint's scheme (http/https), port (e.g. :8080) and any path: only the
	// region segment of the host changes. Regeneration only applies when an S3
	// endpoint is actually configured. If it is empty (the user did not configure
	// one), we must NOT synthesize a host from a stray region override (which may
	// come from a leftover `region` value in state): leaving it empty lets the
	// endpoint resolver fail with the "OOS requires a configured s3 endpoint"
	// error instead of silently targeting oos-<region>.ctyunapi.cn.
	endpoint := c.endpoints[servicePackageName]
	if servicePackageName == names.S3 && endpoint != "" {
		if inContext, ok := FromContext(ctx); ok {
			if override := inContext.OverrideRegion(); override != "" && override != names.RegionForEndpoint(servicePackageName, endpoint) {
				if rebased := names.RebaseEndpointRegion(servicePackageName, endpoint, override); rebased != "" {
					endpoint = rebased
				}
			}
		}
	}

	m := map[string]any{
		"aws_sdkv2_config": c.awsConfig,
		"endpoint":         endpoint,
		"partition":        c.Partition(ctx),
		"region":           c.Region(ctx),
	}

	// original_region carries the real (non-AWS) OOS region used for SigV4
	// signing. The SDK is initialized with a dummy region (see
	// internal/conns/config.go), so each OOS service installs a signing
	// middleware that resets the signing region to this value.
	if effectiveRealRegion != "" && !inttypes.IsAWSRegion(effectiveRealRegion) {
		m["original_region"] = effectiveRealRegion
	}

	switch servicePackageName {
	case names.S3:
		m["s3_use_path_style"] = c.s3UsePathStyle
	}

	return m
}

// client returns the AWS SDK for Go v2 API client for the specified service.
// The default service client (`extra` is empty) is cached. In this case the AWSClient lock is held.
// This function is not a method on `AWSClient` as methods can't be parameterized (https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#no-parameterized-methods).
func client[T any](ctx context.Context, c *AWSClient, servicePackageName string, extra map[string]any) (T, error) {
	ctx = tflog.SetField(ctx, "tf_aws.service_package", servicePackageName)
	region := c.Region(ctx)

	isDefault := len(extra) == 0
	// Default service client is cached.
	if isDefault {
		c.lock.Lock()
		defer c.lock.Unlock() // Runs at function exit, NOT block.

		if v, ok := c.clients[region]; ok {
			if raw, ok := v[servicePackageName]; ok {
				if client, ok := raw.(T); ok {
					return client, nil
				} else {
					zero := inttypes.Zero[T]()
					return zero, fmt.Errorf("AWS SDK v2 API client (%s): %T, want %T", servicePackageName, raw, zero)
				}
			}
		}
	}

	sp := c.ServicePackage(ctx, servicePackageName)
	if sp == nil {
		return inttypes.Zero[T](), fmt.Errorf("unknown service package: %s", servicePackageName)
	}

	v, ok := sp.(interface {
		NewClient(context.Context, map[string]any) (T, error)
	})
	if !ok {
		return inttypes.Zero[T](), fmt.Errorf("no AWS SDK v2 API client factory: %s", servicePackageName)
	}

	config := c.apiClientConfig(ctx, servicePackageName)
	maps.Copy(config, extra) // Extras overwrite per-service defaults.
	client, err := v.NewClient(ctx, config)
	if err != nil {
		return inttypes.Zero[T](), err
	}

	// All customization for AWS SDK for Go v2 API clients must be done during construction.

	if isDefault {
		if _, ok := c.clients[region]; !ok {
			c.clients[region] = make(map[string]any, 0)
		}
		c.clients[region][servicePackageName] = client
	}

	return client, nil
}
