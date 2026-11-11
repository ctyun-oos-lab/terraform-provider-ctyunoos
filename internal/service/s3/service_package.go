// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/vcr"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (p *servicePackage) withExtraOptions(ctx context.Context, config map[string]any) []func(*s3.Options) {
	cfg := *(config["aws_sdkv2_config"].(*aws.Config))

	return []func(*s3.Options){
		func(o *s3.Options) {
			// Inject the real OOS signing region. The SDK is initialized with a
			// dummy AWS region (OOS region names are non-standard), so reset the
			// SigV4 signing region to the real region in effect for this
			// operation (per-resource override > provider region > default).
			if originalRegion, ok := config["original_region"].(string); ok && originalRegion != "" {
				tflog.Debug(ctx, "Injecting original region for OOS endpoint", map[string]any{
					"original_region": originalRegion,
				})
				o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
					return stack.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"InjectOriginalS3Region",
							func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								ctx = awsmiddleware.SetSigningRegion(ctx, originalRegion)
								return next.HandleFinalize(ctx, in)
							},
						),
						middleware.Before,
					)
				})
			}
		},
		func(o *s3.Options) {
			o.UsePathStyle = config["s3_use_path_style"].(bool)
		},
		func(o *s3.Options) {
			o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
				return stack.Build.Add(
					middleware.BuildMiddlewareFunc(
						"RemoveOOSAMZUserAgent",
						func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
							if req, ok := in.Request.(*smithyhttp.Request); ok && req != nil {
								// The provider configuration owns User-Agent composition. OOS does not accept
								// the SDK's auxiliary X-Amz-User-Agent header, so remove only that header.
								req.Header.Del("X-Amz-User-Agent")
							}
							return next.HandleBuild(ctx, in)
						},
					),
					middleware.After,
				)
			})
		},
		func(o *s3.Options) {
			if inContext, ok := conns.FromContext(ctx); ok && inContext.VCREnabled() {
				tflog.Info(ctx, "overriding retry behavior to immediately return VCR errors")
				o.Retryer = conns.AddIsErrorRetryables(cfg.Retryer().(aws.RetryerV2), vcr.InteractionNotFoundRetryableFunc)
			}
		},
	}
}
