// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	OOSIAMUnsignedPayload = "UNSIGNED-PAYLOAD"
	OOSIAMSigningName     = "sts"
)

type OOSIAMHTTPSigner struct {
	Signer        *v4.Signer
	SigningName   string
	SigningRegion string
}

func (s OOSIAMHTTPSigner) SignHTTP(ctx context.Context, credentials aws.Credentials, r *http.Request, _ string, service string, region string, signingTime time.Time, _ ...func(*v4.SignerOptions)) error {
	if signingTime.IsZero() {
		signingTime = time.Now().UTC()
	}

	signingName := s.SigningName
	if signingName == "" {
		signingName = service
	}

	signingRegion := s.SigningRegion
	if signingRegion == "" {
		signingRegion = region
	}

	signer := s.Signer
	if signer == nil {
		signer = v4.NewSigner()
	}

	removedHeaders := map[string][]string{}
	for _, header := range []string{"Amz-Sdk-Invocation-Id", "Amz-Sdk-Request"} {
		if values := r.Header.Values(header); len(values) > 0 {
			removedHeaders[header] = append([]string(nil), values...)
			r.Header.Del(header)
		}
	}

	err := signer.SignHTTP(ctx, credentials, r, OOSIAMUnsignedPayload, signingName, signingRegion, signingTime, func(o *v4.SignerOptions) {
		// Match S3-style signing behavior as closely as possible.
		o.DisableURIPathEscaping = true
	})

	for header, values := range removedHeaders {
		r.Header.Del(header)
		for _, value := range values {
			r.Header.Add(header, value)
		}
	}

	return err
}

func AddOOSIAMUnsignedPayloadHeaderMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"AddOOSIAMUnsignedPayloadHeader",
				func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if ok && req != nil && req.Header.Get("X-Amz-Content-Sha256") == "" {
						req.Header.Set("X-Amz-Content-Sha256", OOSIAMUnsignedPayload)
					}

					return next.HandleFinalize(ctx, in)
				},
			),
			middleware.Before,
		)
	}
}

func ParseOOSIAMSigningRegion(endpoint string) string {
	if endpoint == "" {
		return ""
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}

	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return ""
	}

	labels := strings.Split(parts[0], "-")
	if len(labels) < 2 {
		return ""
	}

	for i := 1; i < len(labels)-1; i++ {
		if labels[i] != "" {
			return labels[i]
		}
	}

	return ""
}
