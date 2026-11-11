// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudtrail

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
)

const (
	oosUnsignedPayload       = "UNSIGNED-PAYLOAD"
	oosCloudTrailSigningName = "cloudtrail"
	// OOS CloudTrail JSON-RPC target prefix (see CloudTrailConfig.targetHeader / CloudTrailAPITestUtils).
	oosCloudTrailTargetPrefix = "cn.ctyunapi.oos-cn-cloudtrail.v20131101.CloudTrail_20131101."
	awsCloudTrailTargetPrefix = "CloudTrail_20131101."

	// millisecondsPerSecond converts epoch seconds to the epoch milliseconds OOS CloudTrail expects.
	millisecondsPerSecond = 1000
)

func (p *servicePackage) withExtraOptions(_ context.Context, config map[string]any) []func(*cloudtrail.Options) {
	signingRegion := parseOOSCloudTrailSigningRegion(config[names.AttrEndpoint].(string))

	return []func(*cloudtrail.Options){
		func(o *cloudtrail.Options) {
			o.HTTPSignerV4 = oosCloudTrailHTTPSigner{
				signer:        v4.NewSigner(),
				signingName:   oosCloudTrailSigningName,
				signingRegion: signingRegion,
			}
			o.APIOptions = append(o.APIOptions,
				addCTYUNOOSUserAgentMiddleware(),
				addOOSCloudTrailHeaderMiddleware(),
				addOOSCloudTrailErrorResponseMiddleware(),
				addOOSCloudTrailTimestampMiddleware(),
			)
		},
	}
}

// OOS returns XML error bodies while the AWS CloudTrail SDK expects AWS JSON 1.1.
// Parse them directly so provider errors retain OOS error codes and messages.
func addOOSCloudTrailErrorResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"OOSCloudTrailErrorResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, metadata, err := next.HandleDeserialize(ctx, in)
					response, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || response == nil || response.Response == nil || response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
						return out, metadata, err
					}

					handled, parsedErr := parseOOSCloudTrailErrorResponse(response, &metadata)
					if !handled {
						return out, metadata, err
					}

					return out, metadata, parsedErr
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func parseOOSCloudTrailErrorResponse(response *smithyhttp.Response, metadata *middleware.Metadata) (bool, error) {
	if response == nil || response.Response == nil || response.Body == nil {
		return false, nil
	}

	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return true, err
	}

	response.Body = io.NopCloser(bytes.NewReader(body))
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '<' {
		return false, nil
	}

	var parsed struct {
		Code      string `xml:"Code"`
		Message   string `xml:"Message"`
		RequestID string `xml:"RequestId"`
	}
	if err := xml.Unmarshal(trimmed, &parsed); err != nil {
		return true, newOOSCloudTrailAPIError(response.StatusCode, "UnknownError", string(trimmed))
	}

	code := strings.TrimSpace(parsed.Code)
	if code == "" {
		return true, newOOSCloudTrailAPIError(response.StatusCode, "UnknownError", string(trimmed))
	}

	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = code
	}
	if requestID := strings.TrimSpace(parsed.RequestID); requestID != "" && metadata != nil {
		awsmiddleware.SetRequestIDMetadata(metadata, requestID)
	}

	return true, newOOSCloudTrailAPIError(response.StatusCode, code, message)
}

func newOOSCloudTrailAPIError(statusCode int, code, message string) error {
	apiErr := &smithy.GenericAPIError{Code: code, Message: message}
	if statusCode == http.StatusNotFound || strings.EqualFold(code, "TrailNotFoundException") {
		return &retry.NotFoundError{LastError: apiErr}
	}

	return apiErr
}

func addCTYUNOOSUserAgentMiddleware() func(*middleware.Stack) error {
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

func addOOSCloudTrailHeaderMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"AddOOSCloudTrailRequestHeaders",
				func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if ok && req != nil {
						if target := req.Header.Get("X-Amz-Target"); target != "" && !strings.Contains(target, oosCloudTrailTargetPrefix) {
							if strings.HasPrefix(target, awsCloudTrailTargetPrefix) {
								action := strings.TrimPrefix(target, awsCloudTrailTargetPrefix)
								req.Header.Set("X-Amz-Target", oosCloudTrailTargetPrefix+action)
							}
						}

						if req.Header.Get("X-Amz-Content-Sha256") == "" {
							req.Header.Set("X-Amz-Content-Sha256", oosUnsignedPayload)
						}

						if err := fixOOSCloudTrailRequestBody(req); err != nil {
							return middleware.FinalizeOutput{}, middleware.Metadata{}, err
						}
					}

					return next.HandleFinalize(ctx, in)
				},
			),
			middleware.Before,
		)
	}
}

type oosCloudTrailHTTPSigner struct {
	signer        *v4.Signer
	signingName   string
	signingRegion string
}

func (s oosCloudTrailHTTPSigner) SignHTTP(ctx context.Context, credentials aws.Credentials, r *http.Request, _ string, service string, region string, signingTime time.Time, _ ...func(*v4.SignerOptions)) error {
	if signingTime.IsZero() {
		signingTime = time.Now().UTC()
	}

	signingName := s.signingName
	if signingName == "" {
		signingName = service
	}

	signingRegion := s.signingRegion
	if signingRegion == "" {
		signingRegion = region
	}

	signer := s.signer
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

	err := signer.SignHTTP(ctx, credentials, r, oosUnsignedPayload, signingName, signingRegion, signingTime, func(o *v4.SignerOptions) {
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

// parseOOSCloudTrailSigningRegion extracts the region from OOS CloudTrail endpoint hostnames
// such as oos-bj-cloudtrail.ctyunapi.cn -> bj.
func parseOOSCloudTrailSigningRegion(endpoint string) string {
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

// oosCloudTrailRequestFieldRewrites maps AWS SDK JSON field names to OOS PascalCase names.
//
// Audited OOS CloudTrail APIs (CloudTrailServer.java) vs AWS SDK v2 serializers:
//   - CreateTrail, DeleteTrail, GetTrailStatus, UpdateTrail, StartLogging, StopLogging: Name, S3BucketName, S3KeyPrefix — match
//   - GetEventSelectors, PutEventSelectors: TrailName, EventSelectors, ReadWriteType — match
//   - DescribeTrails: SDK sends trailNameList/includeShadowTrails; OOS only reads TrailNameList
//   - LookupEvents: field names match; SDK sends StartTime/EndTime as epoch seconds, OOS expects epoch milliseconds (handled below)
var oosCloudTrailRequestFieldRewrites = map[string]string{
	"trailNameList": "TrailNameList",
}

func fixOOSCloudTrailRequestBody(req *smithyhttp.Request) error {
	if req == nil || req.GetStream() == nil {
		return nil
	}

	body, err := io.ReadAll(req.GetStream())
	if err != nil {
		return err
	}

	if len(body) == 0 {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil //nolint:nilerr // non-JSON bodies are left untouched by design
	}

	changed := false

	for awsKey, oosKey := range oosCloudTrailRequestFieldRewrites {
		if v, ok := payload[awsKey]; ok {
			payload[oosKey] = v
			delete(payload, awsKey)
			changed = true
		}
	}

	if strings.Contains(req.Header.Get("X-Amz-Target"), "LookupEvents") {
		for _, key := range []string{"StartTime", "EndTime"} {
			if v, ok := payload[key]; ok {
				if millis, ok := oosCloudTrailEpochSecondsToMillis(v); ok {
					payload[key] = millis
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
	}

	fixed, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	updated, err := req.SetStream(bytes.NewReader(fixed))
	if err != nil {
		return err
	}

	*req = *updated

	return nil
}

// oosCloudTrailEpochSecondsToMillis converts AWS SDK epoch-second request values to OOS millisecond longs.
func oosCloudTrailEpochSecondsToMillis(v any) (int64, bool) {
	switch value := v.(type) {
	case float64:
		if value <= 0 {
			return 0, false
		}

		// Values already in milliseconds (13 digits) are left to the caller to skip.
		if value >= 1_000_000_000_000 {
			return int64(value), true
		}

		return int64(value * millisecondsPerSecond), true
	case json.Number:
		f64, err := value.Float64()
		if err != nil || f64 <= 0 {
			return 0, false
		}

		if f64 >= 1_000_000_000_000 {
			n, err := value.Int64()
			if err != nil {
				return 0, false
			}

			return n, true
		}

		return int64(f64 * millisecondsPerSecond), true
	default:
		return 0, false
	}
}

// OOS CloudTrail returns epoch milliseconds for timestamp fields; the AWS SDK expects epoch seconds.
var oosCloudTrailTimestampFields = map[string]struct{}{
	"EventTime":          {},
	"LatestDeliveryTime": {},
	"StartLoggingTime":   {},
	"StopLoggingTime":    {},
}

func addOOSCloudTrailTimestampMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"FixOOSCloudTrailTimestamps",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					if err != nil {
						return out, md, err
					}

					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Body == nil {
						return out, md, nil
					}

					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						return out, md, err
					}

					if len(body) == 0 {
						return out, md, nil
					}

					fixed, err := convertOOSCloudTrailTimestampsJSON(body)
					if err != nil {
						return out, md, err
					}

					resp.Body = io.NopCloser(bytes.NewReader(fixed))
					resp.ContentLength = int64(len(fixed))

					return out, md, nil
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func convertOOSCloudTrailTimestampsJSON(body []byte) ([]byte, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, nil //nolint:nilerr // non-JSON bodies are returned unchanged by design
	}

	convertOOSCloudTrailTimestampValues(payload)

	return json.Marshal(payload)
}

func convertOOSCloudTrailTimestampValues(v any) {
	switch value := v.(type) {
	case map[string]any:
		for key, nested := range value {
			if _, ok := oosCloudTrailTimestampFields[key]; ok {
				if millis, ok := oosCloudTrailTimestampMillis(nested); ok {
					value[key] = float64(millis) / 1000.0
					continue
				}
			}

			convertOOSCloudTrailTimestampValues(nested)
		}
	case []any:
		for _, nested := range value {
			convertOOSCloudTrailTimestampValues(nested)
		}
	}
}

func oosCloudTrailTimestampMillis(v any) (int64, bool) {
	switch value := v.(type) {
	case float64:
		if value <= 0 {
			return 0, false
		}

		return int64(value), true
	case json.Number:
		n, err := value.Int64()
		if err != nil || n <= 0 {
			return 0, false
		}

		return n, true
	default:
		return 0, false
	}
}
