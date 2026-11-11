// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sdkv2

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log"
	"os"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2/internal/attribute"
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	tfunique "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/unique"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/vcr"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
	"github.com/hashicorp/aws-sdk-go-base/v2/useragent"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	resourceSchemasValidated bool
)

type sdkProvider struct {
	provider        *schema.Provider
	servicePackages iter.Seq2[int, conns.ServicePackage]
}

// providerMeta matches the shape of ProviderMetaSchema
type providerMeta struct {
	UserAgent []string `cty:"user_agent"`
}

func contextWithProviderMeta(ctx context.Context, getProviderMeta getProviderMetaFunc) (context.Context, error) {
	if getProviderMeta == nil {
		return ctx, nil
	}

	var metadata providerMeta
	if err := getProviderMeta(&metadata); err != nil {
		return ctx, fmt.Errorf("getting provider_meta: %w", err)
	}

	if len(metadata.UserAgent) > 0 {
		ctx = useragent.Context(ctx, useragent.FromSlice(metadata.UserAgent))
	}

	return ctx, nil
}

// NewProvider returns a new, initialized Terraform Plugin SDK v2-style provider instance.
// The provider instance is fully configured once the `ConfigureContextFunc` has been called.
func NewProvider(ctx context.Context) (*schema.Provider, error) {
	log.Printf("Creating Terraform AWS Provider (SDKv2-style)...")

	sdkProvider := &sdkProvider{
		provider: &schema.Provider{
			// This schema must match exactly the Terraform Protocol v6 (Terraform Plugin Framework) provider's schema.
			// Notably the attributes can have no Default values.
			Schema: map[string]*schema.Schema{
				"access_key": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "The access key for API operations. You can retrieve this\n" +
						"from the 'Security & Credentials' section of the OOS console. " +
						"Can also be sourced from the CTYUNOOS_ACCESS_KEY environment variable.",
				},
				"assume_role": assumeRoleSchema(),
				"custom_ca_bundle": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "File containing custom root and intermediate certificates. " +
						"Can also be configured using the `CTYUNOOS_CA_BUNDLE` environment variable. " +
						"(Setting `ca_bundle` in the shared config file is not supported.)",
				},
				"endpoints": endpointsSchema(),
				"http_proxy": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "URL of a proxy to use for HTTP requests when accessing the OOS API. " +
						"Can also be set using the `HTTP_PROXY` or `http_proxy` environment variables.",
				},
				"https_proxy": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "URL of a proxy to use for HTTPS requests when accessing the OOS API. " +
						"Can also be set using the `HTTPS_PROXY` or `https_proxy` environment variables.",
				},
				"insecure": {
					Type:     schema.TypeBool,
					Optional: true,
					Description: "Explicitly allow the provider to perform \"insecure\" SSL requests. If omitted, " +
						"default value is `false`",
				},
				"max_retries": {
					Type:     schema.TypeInt,
					Optional: true,
					Description: "The maximum number of times an OOS API request is\n" +
						"being executed. If the API request still fails, an error is\n" +
						"thrown.",
				},
				"no_proxy": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "Comma-separated list of hosts that should not use HTTP or HTTPS proxies. " +
						"Can also be set using the `NO_PROXY` or `no_proxy` environment variables.",
				},
				"retry_mode": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "Specifies how retries are attempted. Valid values are `standard` and `adaptive`. " +
						"Can also be configured using the `CTYUNOOS_RETRY_MODE` environment variable.",
				},
				"s3_use_path_style": {
					Type:     schema.TypeBool,
					Optional: true,
					Default:  true,
					Description: "Set this to true to enable the request to use path-style addressing,\n" +
						"i.e., https://<s3-host>:<port>/BUCKET/KEY. By default, the S3 client will\n" +
						"use virtual hosted bucket addressing when possible\n" +
						"(https://BUCKET.<s3-host>:<port>/KEY). Specific to the OOS S3 service. Defaults to true.",
				},
				"secret_key": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "The secret key for API operations. You can retrieve this\n" +
						"from the 'Security & Credentials' section of the OOS console. " +
						"Can also be sourced from the CTYUNOOS_SECRET_KEY environment variable.",
				},
				"security_token": {
					Type:     schema.TypeString,
					Optional: true,
					Description: "session token. A session token is only required if you are\n" +
						"using temporary security credentials. " +
						"Can also be sourced from the CTYUNOOS_SESSION_TOKEN environment variable.",
				},
				"token_bucket_rate_limiter_capacity": {
					Type:        schema.TypeInt,
					Optional:    true,
					Description: "The capacity of the SDK's token bucket rate limiter.",
				},
				"user_agent": {
					Type:        schema.TypeList,
					Optional:    true,
					Description: "Product details to use in the User-Agent string sent in all CTYUN OOS API calls. This replaces the default `terraform-provider-ctyunoos/<version>` identifier.",
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
			},

			// ProviderMetaSchema enables module-scoped User-Agent modifications
			ProviderMetaSchema: map[string]*schema.Schema{
				"user_agent": {
					Type:        schema.TypeList,
					Optional:    true,
					Description: "Product details to append to the User-Agent string sent in all CTYUN OOS API calls.",
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
			},

			// Data sources and resources implemented using Terraform Plugin SDK
			// should use the @SDKDataSource and @SDKResource function-level annotations
			// rather than adding directly to these maps.
			DataSourcesMap: make(map[string]*schema.Resource),
			ResourcesMap:   make(map[string]*schema.Resource),
		},
		servicePackages: slices.All(servicePackages(ctx)),
	}

	sdkProvider.provider.ConfigureContextFunc = sdkProvider.configure

	// Acceptance tests call this function multiple times, potentially in parallel.
	// To avoid "fatal error: concurrent map writes", take a lock.
	const (
		mutexKVKey = "provider.New"
	)
	conns.GlobalMutexKV.Lock(mutexKVKey)
	defer conns.GlobalMutexKV.Unlock(mutexKVKey)

	// Because we try and share resource schemas as much as possible,
	// we need to ensure that we only validate the resource schemas once.
	if !resourceSchemasValidated {
		if err := sdkProvider.validateResourceSchemas(ctx); err != nil {
			return nil, err
		}

		resourceSchemasValidated = true
	}

	servicePackageMap, err := sdkProvider.initialize(ctx)

	if err != nil {
		return nil, err
	}

	// Set the provider Meta (instance data) here.
	// It will be overwritten by the result of the call to ConfigureContextFunc,
	// but can be used pre-configuration by other (non-primary) provider servers.
	var c *conns.AWSClient
	if v, ok := sdkProvider.provider.Meta().(*conns.AWSClient); ok {
		c = v
	} else {
		c = new(conns.AWSClient)
	}
	c.SetServicePackages(ctx, servicePackageMap)
	sdkProvider.provider.SetMeta(c)

	return sdkProvider.provider, nil
}

// configure ensures that the provider is fully configured.
func (p *sdkProvider) configure(ctx context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
	var diags diag.Diagnostics

	terraformVersion := p.provider.TerraformVersion
	if terraformVersion == "" {
		// Terraform 0.12 introduced this field to the protocol
		// We can therefore assume that if it's missing it's 0.10 or 0.11
		terraformVersion = "0.11+compatible"
	}

	config := conns.Config{
		AccessKey:                      d.Get("access_key").(string),
		CustomCABundle:                 d.Get("custom_ca_bundle").(string),
		Endpoints:                      make(map[string]string),
		Insecure:                       d.Get("insecure").(bool),
		MaxRetries:                     5, // Set default here, not in schema (muxing with v6 provider).
		S3UsePathStyle:                 d.Get("s3_use_path_style").(bool),
		SecretKey:                      d.Get("secret_key").(string),
		TerraformVersion:               terraformVersion,
		Token:                          d.Get("security_token").(string),
		TokenBucketRateLimiterCapacity: d.Get("token_bucket_rate_limiter_capacity").(int),
	}

	if v, ok := d.Get("retry_mode").(string); ok && v != "" {
		mode, err := aws.ParseRetryMode(v)
		if err != nil {
			return nil, sdkdiag.AppendFromErr(diags, err)
		}
		config.RetryMode = mode
	}

	if v, ok := d.GetOk("assume_role"); ok {
		path := cty.GetAttrPath("assume_role")
		v := v.([]any)
		if len(v) == 1 {
			if v[0] == nil {
				diags = append(diags, errs.NewAttributeRequiredWillBeError(path.IndexInt(0), "role_arn"))
			} else {
				l := v[0].(map[string]any)
				if s, ok := l["role_arn"]; !ok || s == "" {
					diags = append(diags, errs.NewAttributeRequiredWillBeError(path.IndexInt(0), "role_arn"))
				} else {
					ar, dg := expandAssumeRoles(ctx, path, v)
					diags = append(diags, dg...)
					if dg.HasError() {
						return nil, diags
					}
					config.AssumeRole = ar
				}
			}
		} else if len(v) > 1 {
			ar, dg := expandAssumeRoles(ctx, path, v)
			diags = append(diags, dg...)
			if dg.HasError() {
				return nil, diags
			}
			config.AssumeRole = ar
		}
	}

	v := d.Get("endpoints")
	endpoints, dx := expandEndpoints(ctx, v.(*schema.Set).List())
	diags = append(diags, dx...)
	if diags.HasError() {
		return nil, diags
	}
	config.Endpoints = endpoints

	// Fallback for OOS custom endpoint environment variables: takes effect only when the tf block/variable does not specify an endpoint for the service.
	expandEndpointsCustomEnvVars(config.Endpoints, &diags)
	if diags.HasError() {
		return nil, diags
	}

	// Back-fill from OOS custom credential environment variables: when the tf block/variable does not specify them, read ak/sk/token from CTYUNOOS_*.
	expandCredentialsCustomEnvVars(&config)

	if v, ok := d.GetOkExists("http_proxy"); ok {
		if s, sok := v.(string); sok {
			config.HTTPProxy = aws.String(s)
		}
	}
	if v, ok := d.GetOkExists("https_proxy"); ok {
		if s, sok := v.(string); sok {
			config.HTTPSProxy = aws.String(s)
		}
	}

	if v, ok := d.Get("no_proxy").(string); ok && v != "" {
		config.NoProxy = v
	}

	if v, ok := d.GetOk("max_retries"); ok {
		config.MaxRetries = v.(int)
	}

	if v, ok := d.GetOk("user_agent"); ok && len(v.([]any)) > 0 {
		config.UserAgent = useragent.FromSlice(v.([]any))
	}

	var c *conns.AWSClient
	if v, ok := p.provider.Meta().(*conns.AWSClient); ok {
		c = v
	} else {
		c = new(conns.AWSClient)
	}
	c, ds := config.ConfigureProvider(ctx, c)
	diags = append(diags, ds...)

	if diags.HasError() {
		return nil, diags
	}

	return c, diags
}

// expandCredentialsCustomEnvVars only reads OOS credential environment
// variables when no credential field was explicitly configured. AK, SK, and
// session token form one credential set and must not be combined by source.
func expandCredentialsCustomEnvVars(config *conns.Config) {
	if config.AccessKey != "" || config.SecretKey != "" || config.Token != "" {
		return
	}

	config.AccessKey = os.Getenv("CTYUNOOS_ACCESS_KEY")
	config.SecretKey = os.Getenv("CTYUNOOS_SECRET_KEY")
	config.Token = os.Getenv("CTYUNOOS_SESSION_TOKEN")
}

// initialize is called from `New` to perform any Terraform Plugin SDK v2-style initialization.
func (p *sdkProvider) initialize(ctx context.Context) (map[string]conns.ServicePackage, error) {
	log.Printf("Initializing Terraform AWS Provider (SDKv2-style)...")

	var errs []error
	servicePackageMap := make(map[string]conns.ServicePackage)

	for _, sp := range p.servicePackages {
		servicePackageName := sp.ServicePackageName()
		servicePackageMap[servicePackageName] = sp

		for _, v := range sp.SDKDataSources(ctx) {
			typeName := v.TypeName

			if _, ok := p.provider.DataSourcesMap[typeName]; ok {
				errs = append(errs, fmt.Errorf("duplicate data source: %s", typeName))
				continue
			}

			r := v.Factory()

			// Ensure that the correct CRUD handler variants are used.
			if r.Read != nil || r.ReadContext != nil {
				errs = append(errs, fmt.Errorf("incorrect Read handler variant: %s data source", typeName))
				continue
			}

			var isRegionOverrideEnabled bool
			if v := v.Region; !tfunique.IsHandleNil(v) && v.Value().IsOverrideEnabled {
				isRegionOverrideEnabled = true
			}

			var interceptors interceptorInvocations

			if isRegionOverrideEnabled {
				v := v.Region.Value()
				s := r.SchemaMap()

				if _, ok := s[names.AttrRegion]; !ok {
					// Inject a top-level "region" attribute.
					regionSchema := attribute.Region()

					if f := r.SchemaFunc; f != nil {
						r.SchemaFunc = func() map[string]*schema.Schema {
							s := f()
							s[names.AttrRegion] = regionSchema
							return s
						}
					} else {
						r.Schema[names.AttrRegion] = regionSchema
					}
				}

				if v.IsValidateOverrideInPartition {
					interceptors = append(interceptors, interceptorInvocation{
						when:        Before,
						why:         Read,
						interceptor: dataSourceValidateRegion(),
					})
				}
				interceptors = append(interceptors, interceptorInvocation{
					when:        After,
					why:         Read,
					interceptor: setRegionInState(),
				})
			}

			if !tfunique.IsHandleNil(v.Tags) {
				interceptors = append(interceptors, interceptorInvocation{
					when:        Before | After,
					why:         Read,
					interceptor: dataSourceTransparentTagging(v.Tags),
				})
			}

			opts := wrappedDataSourceOptions{
				bootstrapContext: func(ctx context.Context, getAttribute getAttributeFunc, getProviderMeta getProviderMetaFunc, meta any) (context.Context, error) {
					var overrideRegion string

					if isRegionOverrideEnabled && getAttribute != nil {
						if region, ok := getAttribute(names.AttrRegion); ok {
							overrideRegion = region.(string)
						}
					}

					ctx = conns.NewResourceContext(ctx, servicePackageName, v.Name, v.TypeName, overrideRegion)
					if c, ok := meta.(*conns.AWSClient); ok {
						ctx = tftags.NewContext(ctx, c.DefaultTagsConfig(ctx), c.IgnoreTagsConfig(ctx), c.TagPolicyConfig(ctx))
						ctx = c.RegisterLogger(ctx)
					}

					ctx, err := contextWithProviderMeta(ctx, getProviderMeta)
					if err != nil {
						return ctx, err
					}

					return ctx, nil
				},
				interceptors: interceptors,
				typeName:     typeName,
			}
			wrapDataSource(r, opts)
			p.provider.DataSourcesMap[typeName] = r
		}

		for _, resource := range sp.SDKResources(ctx) {
			typeName := resource.TypeName

			if _, ok := p.provider.ResourcesMap[typeName]; ok {
				errs = append(errs, fmt.Errorf("duplicate resource: %s", typeName))
				continue
			}

			r := resource.Factory()

			// Ensure that the correct CRUD handler variants are used.
			if r.Create != nil || r.CreateContext != nil {
				errs = append(errs, fmt.Errorf("incorrect Create handler variant: %s resource", typeName))
				continue
			}
			if r.Read != nil || r.ReadContext != nil {
				errs = append(errs, fmt.Errorf("incorrect Read handler variant: %s resource", typeName))
				continue
			}
			if r.Update != nil || r.UpdateContext != nil {
				errs = append(errs, fmt.Errorf("incorrect Update handler variant: %s resource", typeName))
				continue
			}
			if r.Delete != nil || r.DeleteContext != nil {
				errs = append(errs, fmt.Errorf("incorrect Delete handler variant: %s resource", typeName))
				continue
			}

			var isRegionOverrideEnabled bool
			if v := resource.Region; !tfunique.IsHandleNil(v) && v.Value().IsOverrideEnabled {
				isRegionOverrideEnabled = true
			}

			var interceptors interceptorInvocations

			if isRegionOverrideEnabled {
				v := resource.Region.Value()
				s := r.SchemaMap()

				if _, ok := s[names.AttrRegion]; !ok {
					// Inject a top-level "region" attribute.
					regionSchema := attribute.Region()

					// If the resource defines no Update handler then add a stub to fake out 'Provider.Validate'.
					if r.UpdateWithoutTimeout == nil {
						r.UpdateWithoutTimeout = schema.NoopContext
					}

					if f := r.SchemaFunc; f != nil {
						r.SchemaFunc = func() map[string]*schema.Schema {
							s := f()
							s[names.AttrRegion] = regionSchema
							return s
						}
					} else {
						r.Schema[names.AttrRegion] = regionSchema
					}
				}

				if v.IsValidateOverrideInPartition {
					interceptors = append(interceptors, interceptorInvocation{
						when:        Before,
						why:         CustomizeDiff,
						interceptor: resourceValidateRegion(),
					})
				}
				interceptors = append(interceptors, interceptorInvocation{
					when:        Before,
					why:         CustomizeDiff,
					interceptor: defaultRegion(),
				})
				interceptors = append(interceptors, interceptorInvocation{
					when:        After,
					why:         Read,
					interceptor: setRegionInState(),
				})
				interceptors = append(interceptors, resourceImportRegion())
			}

			if !tfunique.IsHandleNil(resource.Tags) {
				interceptors = append(interceptors, interceptorInvocation{
					when:        Before | After | Finally,
					why:         Create | Read | Update,
					interceptor: resourceTransparentTagging(resource.Tags),
				})
				interceptors = append(interceptors, interceptorInvocation{
					when:        Before,
					why:         CustomizeDiff,
					interceptor: validateRequiredTags(),
				})
			}

			if len(resource.Identity.Attributes) > 0 {
				r.Identity = newResourceIdentity(resource.Identity)

				if resource.Identity.IsMutable {
					r.ResourceBehavior.MutableIdentity = true
				}

				interceptors = append(interceptors, newIdentityInterceptor(&resource.Identity))
			}

			if resource.Import.CustomImport {
				if r.Importer == nil || r.Importer.StateContext == nil {
					errs = append(errs, fmt.Errorf("resource type %s: uses CustomImport but does not define an import function", typeName))
					continue
				}

				customResourceImporter(r, &resource.Identity, &resource.Import)
			}
			if resource.Import.WrappedImport {
				if r.Importer != nil && r.Importer.StateContext != nil {
					errs = append(errs, fmt.Errorf("resource type %s: uses WrappedImport but defines an import function", typeName))
					continue
				}

				if resource.Identity.IsARN {
					r.Importer = arnIdentityResourceImporter(resource.Identity)
				} else if resource.Identity.IsSingleton {
					r.Importer = singletonIdentityResourceImporter(resource.Identity)
				} else if resource.Identity.IsCustomInherentRegion {
					r.Importer = customInherentRegionResourceImporter(resource.Identity)
				} else {
					r.Importer = newParameterizedIdentityImporter(resource.Identity, resource.Import)
				}
			}

			opts := wrappedResourceOptions{
				// bootstrapContext is run on all wrapped methods before any interceptors.
				bootstrapContext: func(ctx context.Context, getAttribute getAttributeFunc, getProviderMeta getProviderMetaFunc, meta any) (context.Context, error) {
					var overrideRegion string

					if isRegionOverrideEnabled && getAttribute != nil {
						if region, ok := getAttribute(names.AttrRegion); ok && region != nil {
							overrideRegion = region.(string)
						}
					}

					ctx = conns.NewResourceContext(ctx, servicePackageName, resource.Name, resource.TypeName, overrideRegion)
					if c, ok := meta.(*conns.AWSClient); ok {
						ctx = tftags.NewContext(ctx, c.DefaultTagsConfig(ctx), c.IgnoreTagsConfig(ctx), c.TagPolicyConfig(ctx))
						ctx = c.RegisterLogger(ctx)
						if s := c.RandomnessSource(); s != nil {
							ctx = vcr.NewContext(ctx, s)
						}
					}

					ctx, err := contextWithProviderMeta(ctx, getProviderMeta)
					if err != nil {
						return ctx, err
					}

					return ctx, nil
				},
				interceptors: interceptors,
				typeName:     typeName,
			}
			wrapResource(r, opts)
			p.provider.ResourcesMap[typeName] = r
		}
	}

	return servicePackageMap, errors.Join(errs...)
}

// validateResourceSchemas is called from `New` to validate Terraform Plugin SDK v2-style resource schemas.
func (p *sdkProvider) validateResourceSchemas(ctx context.Context) error {
	var errs []error

	for _, sp := range p.servicePackages {
		for _, v := range sp.SDKDataSources(ctx) {
			typeName := v.TypeName
			r := v.Factory()
			s := r.SchemaMap()

			if v := v.Region; !tfunique.IsHandleNil(v) && v.Value().IsOverrideEnabled {
				if _, ok := s[names.AttrRegion]; ok {
					errs = append(errs, fmt.Errorf("`%s` attribute is defined: %s data source", names.AttrRegion, typeName))
					continue
				}
			}

			if !tfunique.IsHandleNil(v.Tags) {
				// The data source has opted in to transparent tagging.
				// Ensure that the schema look OK.
				if v, ok := s[names.AttrTags]; ok {
					if !v.Computed {
						errs = append(errs, fmt.Errorf("`%s` attribute must be Computed: %s data source", names.AttrTags, typeName))
						continue
					}
				} else {
					errs = append(errs, fmt.Errorf("no `%s` attribute defined in schema: %s data source", names.AttrTags, typeName))
					continue
				}
			}
		}

		for _, resource := range sp.SDKResources(ctx) {
			typeName := resource.TypeName
			r := resource.Factory()
			s := r.SchemaMap()

			if v := resource.Region; !tfunique.IsHandleNil(v) && v.Value().IsOverrideEnabled {
				if _, ok := s[names.AttrRegion]; ok {
					errs = append(errs, fmt.Errorf("`%s` attribute is defined: %s resource", names.AttrRegion, typeName))
					continue
				}
			}

			if !tfunique.IsHandleNil(resource.Tags) {
				// The resource has opted in to transparent tagging.
				// Ensure that the schema look OK.
				if v, ok := s[names.AttrTags]; ok {
					if v.Computed {
						errs = append(errs, fmt.Errorf("`%s` attribute cannot be Computed: %s resource", names.AttrTags, typeName))
						continue
					}
				} else {
					errs = append(errs, fmt.Errorf("no `%s` attribute defined in schema: %s resource", names.AttrTags, typeName))
					continue
				}
			}

			if resource.Identity.IsCustomInherentRegion {
				if resource.Identity.IsGlobalResource {
					errs = append(errs, fmt.Errorf("`IsCustomInherentRegion` is not supported for Global resources: %s resource", typeName))
					continue
				}
			}
		}
	}

	return errors.Join(errs...)
}

func assumeRoleSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"duration": {
					Type:         schema.TypeString,
					Optional:     true,
					Description:  "The duration, between 15 minutes and 12 hours, of the role session. Valid time units are ns, us (or µs), ms, s, h, or m.",
					ValidateFunc: validAssumeRoleDuration,
				},
				"external_id": {
					Type:         schema.TypeString,
					Optional:     true,
					Description:  "A unique identifier that might be required when you assume a role in another account.",
					ValidateFunc: validAssumeRoleExternalID,
				},
				"policy": {
					Type:         schema.TypeString,
					Optional:     true,
					Description:  "IAM Policy JSON describing further restricting permissions for the IAM Role being assumed.",
					ValidateFunc: validAssumeRolePolicy,
				},
				"role_arn": {
					Type:         schema.TypeString,
					Optional:     true, // For historical reasons, we allow an empty `assume_role` block
					Description:  "ARN of an IAM Role to assume prior to making API calls.",
					ValidateFunc: validAssumeRoleRoleARN,
				},
				"session_name": {
					Type:     schema.TypeString,
					Optional: true,
					DefaultFunc: func() (any, error) {
						return fmt.Sprintf("terraform-oos-%d", time.Now().UnixNano()), nil
					},
					Description:  "An identifier for the assumed role session.",
					ValidateFunc: validAssumeRoleSessionName,
				},
				"source_identity": {
					Type:         schema.TypeString,
					Optional:     true,
					Description:  "Source identity specified by the principal assuming the role.",
					ValidateFunc: validAssumeRoleSourceIdentity,
				},
			},
		},
	}
}

func expandAssumeRoles(ctx context.Context, path cty.Path, tfList []any) (result []awsbase.AssumeRole, diags diag.Diagnostics) {
	result = make([]awsbase.AssumeRole, len(tfList))

	for i, v := range tfList {
		path := path.IndexInt(i)
		if ar, ok := v.(map[string]any); ok {
			x, d := expandAssumeRole(ctx, path, ar)
			diags = append(diags, d...)
			if d.HasError() {
				return result, diags
			}
			result[i] = x
			tflog.Info(ctx, "assume_role configuration set", map[string]any{
				"tf_aws.assume_role.index":           i,
				"tf_aws.assume_role.role_arn":        result[i].RoleARN,
				"tf_aws.assume_role.session_name":    result[i].SessionName,
				"tf_aws.assume_role.external_id":     result[i].ExternalID,
				"tf_aws.assume_role.source_identity": result[i].SourceIdentity,
			})
		} else {
			return result, append(diags, errs.NewAttributeRequiredError(path, "role_arn"))
		}
	}

	return result, diags
}

func expandAssumeRole(_ context.Context, path cty.Path, tfMap map[string]any) (result awsbase.AssumeRole, diags diag.Diagnostics) {
	if v, ok := tfMap["role_arn"].(string); ok && v != "" {
		result.RoleARN = v
	} else {
		return result, append(diags, errs.NewAttributeRequiredError(path, "role_arn"))
	}

	if v, ok := tfMap["duration"].(string); ok && v != "" {
		duration, _ := time.ParseDuration(v)
		result.Duration = duration
	}

	if v, ok := tfMap["external_id"].(string); ok && v != "" {
		result.ExternalID = v
	}

	if v, ok := tfMap["policy"].(string); ok && v != "" {
		result.Policy = v
	}

	if v, ok := tfMap["session_name"].(string); ok && v != "" {
		result.SessionName = v
	}

	if v, ok := tfMap["source_identity"].(string); ok && v != "" {
		result.SourceIdentity = v
	}

	return result, diags
}
