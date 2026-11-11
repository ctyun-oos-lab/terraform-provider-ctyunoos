// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
)

func (p *servicePackage) withExtraOptions(_ context.Context, config map[string]any) []func(*iam.Options) {
	signingRegion := conns.ParseOOSIAMSigningRegion(config[names.AttrEndpoint].(string))

	return []func(*iam.Options){
		func(o *iam.Options) {
			o.Retryer = retry.AddWithErrorCodes(o.Retryer, oosIAMInvalidHTTPResponseCode)
			o.HTTPSignerV4 = conns.OOSIAMHTTPSigner{
				SigningName:   conns.OOSIAMSigningName,
				SigningRegion: signingRegion,
			}
			o.APIOptions = append(o.APIOptions,
				conns.AddOOSIAMUnsignedPayloadHeaderMiddleware(),
				addOOSIAMUserAgentMiddleware(),
				addOOSIAMErrorResponseMiddleware(),
			)
		},
	}
}

func addOOSIAMUserAgentMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
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
	}
}
