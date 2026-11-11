// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

// OOS-specific operations that the AWS SDK does not natively support.
//
// OOS reuses the S3 protocol but its PutBucket request body and
// GetBucketLocation response body use XML shapes that the generated SDK
// serializer/deserializer cannot produce or parse. Rather than building,
// signing and sending the HTTP request by hand, these operations piggyback on
// the closest standard S3 operation (CreateBucket / GetBucketLocation) and
// install request-scoped middleware that swaps only the request body or the
// response decoder. This way the call still runs through the full SDK pipeline
// (SigV4 signing, the s3 package's InjectOriginalS3Region middleware,
// path-style, endpoint resolution, retry, VCR) instead of re-implementing any
// of it. Keep all such OOS extension calls in this file so they stay
// physically isolated from the reused standard S3 implementation.

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// oosXMLNamespace is the XML namespace OOS expects on the request body, matching
// the standard S3 document namespace.
const oosXMLNamespace = "http://s3.amazonaws.com/doc/2006-03-01/"

// OOS PutBucket request parameter defaults. See docs/oos-development/api/PutBucket.
const (
	oosDefaultRegionType       = "Local"   // RegionType default when omitted.
	oosDefaultScheduleStrategy = "Allowed" // Strategy default when omitted.
	oosDefaultACL              = "private" // x-amz-acl default when omitted.
)

// oosCreateBucketConfiguration is the OOS-specific CreateBucket request body.
// It deliberately does not reuse the SDK's types.CreateBucketConfiguration,
// whose serializer only emits the standard S3 elements.
type oosCreateBucketConfiguration struct {
	XMLName                    xml.Name                       `xml:"CreateBucketConfiguration"`
	XMLNS                      string                         `xml:"xmlns,attr"`
	MetadataLocationConstraint *oosMetadataLocationConstraint `xml:"MetadataLocationConstraint,omitempty"`
	DataLocationConstraint     *oosDataLocationConstraint     `xml:"DataLocationConstraint,omitempty"`
}

type oosMetadataLocationConstraint struct {
	Location string `xml:"Location"`
}

type oosDataLocationConstraint struct {
	Type             string   `xml:"Type,omitempty"`
	LocationList     []string `xml:"LocationList>Location,omitempty"`
	ScheduleStrategy string   `xml:"ScheduleStrategy,omitempty"`
}

// oosCreateBucketInput holds the values used to build the OOS PutBucket request.
type oosCreateBucketInput struct {
	bucket            string
	acl               string   // "" => x-amz-acl omitted (server default "private")
	metadataLocation  *string  // nil => omit MetadataLocationConstraint
	dataType          string   // "" => omit Type (server default "Local")
	dataLocations     []string // nil/empty => omit LocationList
	dataScheduleStreg string   // "" => omit ScheduleStrategy (server default "Allowed")
	hasDataConstraint bool     // false => omit DataLocationConstraint entirely
}

// oosBucketConfiguration is the OOS GetBucketLocation response body. OOS returns
// the bucket's metadata/data location constraints under a <BucketConfiguration>
// root, which the standard S3 GetBucketLocation (returning <LocationConstraint>)
// cannot represent. See docs/oos-development/api/GetBucketLoacation.
type oosBucketConfiguration struct {
	XMLName                    xml.Name                          `xml:"BucketConfiguration"`
	MetadataLocationConstraint *oosMetadataLocationConstraintOut `xml:"MetadataLocationConstraint"`
	DataLocationConstraint     *oosDataLocationConstraintOut     `xml:"DataLocationConstraint"`
}

type oosMetadataLocationConstraintOut struct {
	Location string `xml:"Location"`
}

type oosDataLocationConstraintOut struct {
	Type             string   `xml:"Type"`
	Locations        []string `xml:"LocationList>Location"`
	ScheduleStrategy string   `xml:"ScheduleStrategy"`
}

// marshalCreateBucketBody renders the OOS PutBucket request body XML.
func marshalCreateBucketBody(in oosCreateBucketInput) ([]byte, error) {
	cfg := oosCreateBucketConfiguration{XMLNS: oosXMLNamespace}

	if in.metadataLocation != nil {
		cfg.MetadataLocationConstraint = &oosMetadataLocationConstraint{
			Location: *in.metadataLocation,
		}
	}

	if in.hasDataConstraint {
		cfg.DataLocationConstraint = &oosDataLocationConstraint{
			Type:             in.dataType,
			LocationList:     in.dataLocations,
			ScheduleStrategy: in.dataScheduleStreg,
		}
	}

	body, err := xml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling OOS PutBucket body: %w", err)
	}

	return body, nil
}

// oosPutBucket creates or updates a bucket using the OOS-specific PutBucket
// request. Per the OOS API, PUT Bucket both creates a new bucket and modifies an
// existing bucket's ACL/data position, so Create and Update reuse this call.
//
// It piggybacks on the standard s3.CreateBucket operation (also a PUT /{bucket}
// with an x-amz-acl header) so the request inherits the full SDK pipeline
// (SigV4 signing, region injection, path-style, endpoint resolution, retry).
// CreateBucketConfiguration is left nil so the SDK serializer writes no body;
// a Serialize-step middleware then installs the OOS XML body in its place.
func oosPutBucket(ctx context.Context, conn *s3.Client, in oosCreateBucketInput) error {
	body, err := marshalCreateBucketBody(in)
	if err != nil {
		return err
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(in.bucket),
	}
	if in.acl != "" {
		// Carried as the x-amz-acl header by the SDK serializer, so SigV4 covers
		// it (OOS rejects requests whose signed header set omits x-amz-acl).
		input.ACL = types.BucketCannedACL(in.acl)
	}

	_, err = conn.CreateBucket(ctx, input, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
	})

	return err
}

// replaceRequestBodyMiddleware returns an API option that swaps the serialized
// request body for the given bytes. It runs after the operation serializer so
// it overwrites whatever the SDK produced; the body is a *bytes.Reader, so the
// downstream Build and Finalize steps compute Content-Length and the SigV4
// payload hash automatically.
func replaceRequestBodyMiddleware(body []byte) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Serialize.Add(
			middleware.SerializeMiddlewareFunc("OOSReplaceRequestBody",
				func(ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler) (middleware.SerializeOutput, middleware.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if !ok {
						return next.HandleSerialize(ctx, in)
					}

					r, err := req.SetStream(bytes.NewReader(body))
					if err != nil {
						return middleware.SerializeOutput{}, middleware.Metadata{}, fmt.Errorf("setting OOS request body: %w", err)
					}
					r.Header.Set("Content-Type", "application/xml")
					in.Request = r

					return next.HandleSerialize(ctx, in)
				}),
			middleware.After,
		)
	}
}

// oosGetBucketLocation retrieves a bucket's metadata/data location constraints
// via the OOS-specific GET /{bucket}?location request and parses the
// <BucketConfiguration> response. Returns (nil, nil) when the bucket has no
// configuration body.
//
// It piggybacks on the standard s3.GetBucketLocation operation (also a
// GET /{bucket}?location), swapping out the response deserializer for one that
// reads the OOS <BucketConfiguration> shape instead of <LocationConstraint>.
func oosGetBucketLocation(ctx context.Context, conn *s3.Client, bucket string) (*oosBucketConfiguration, error) {
	var result *oosBucketConfiguration

	_, err := conn.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSBucketLocationDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetBucketLocation response: %w", err)
						}

						// Swapping out OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						// The replaced OperationDeserializer was responsible for
						// populating out.Result; without it the SDK's top-level
						// result.(*GetBucketLocationOutput) assertion panics on a
						// nil interface. We parse the body into our own struct, so
						// a zero-value output is enough to satisfy that assertion.
						out.Result = &s3.GetBucketLocationOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var cfg oosBucketConfiguration
						if err := xml.Unmarshal(data, &cfg); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetBucketLocation response: %w", err)
						}
						result = &cfg

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// parseOOSError decodes an S3-style XML error body into a smithy.APIError so
// that callers' tfawserr/errs error-code checks keep working. Falls back to a
// generic error carrying the HTTP status when the body is not parseable.
func parseOOSError(statusCode int, data []byte) error {
	var apiErr struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.Unmarshal(data, &apiErr); err == nil && apiErr.Code != "" {
		return &smithy.GenericAPIError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
		}
	}

	return fmt.Errorf("OOS API error (HTTP %d): %s", statusCode, bytes.TrimSpace(data))
}

// oosPutBucketLifecycle sends an OOS PUT Bucket Lifecycle request. OOS uses the
// legacy S3 lifecycle XML format, which differs from the SDK's standard-S3
// serialization in two ways:
//
//  1. Request body shape: <Prefix> is a direct child of <Rule> (not nested under
//     <Filter>), and a rule supports only ID / Prefix / Status / Expiration
//     (Days|Date) / Transition (Days|Date, StorageClass, IsAccessTime). The body
//     is marshaled by marshalLifecycleBody and installed via
//     replaceRequestBodyMiddleware, overwriting whatever the SDK would emit. We
//     piggyback on s3.PutBucketLifecycleConfiguration purely for its
//     PUT /{bucket}?lifecycle wiring (signing, endpoint, retry).
//
//  2. Checksum: the SDK defaults to x-amz-checksum-crc32 for this operation and
//     RequireChecksum=true ignores RequestChecksumCalculation, so it cannot be
//     turned off via client options. OOS only accepts the legacy Content-MD5
//     header, which withContentMD5 installs (see its doc comment).
func oosPutBucketLifecycle(ctx context.Context, conn *s3.Client, bucket string, rules []oosLifecycleRule) error {
	body, err := marshalLifecycleBody(rules)
	if err != nil {
		return err
	}

	_, err = conn.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
		withContentMD5(o)
	})

	return err
}

// withContentMD5 swaps the SDK's default flexible-checksum mechanism (which
// emits x-amz-checksum-crc32 / x-amz-sdk-checksum-algorithm headers) for the
// legacy Content-MD5 header that OOS requires.
//
// For checksum-required S3 operations such as DeleteObjects, the generated SDK
// stack installs SetupInputContext / RequestMetricsTracking /
// ComputeInputPayloadChecksum and also removes the smithy-go Build middleware
// whose ID is ContentChecksum. Remove that full chain first, then re-add
// smithy-go's Content-MD5 middleware so it runs in the Build step before SigV4
// signing.
func withContentMD5(o *s3.Options) {
	o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
		// Best-effort removal: these middleware may be absent depending on the
		// operation, so ignore "not found" errors from Remove.
		_, _ = stack.Initialize.Remove("AWSChecksum:SetupInputContext")
		_, _ = stack.Build.Remove("AWSChecksum:RequestMetricsTracking")
		_, _ = stack.Build.Remove("ContentChecksum")
		_, _ = stack.Finalize.Remove("AWSChecksum:ComputeInputPayloadChecksum")
		_, _ = stack.Finalize.Remove("addInputChecksumTrailer")
		return smithyhttp.AddContentChecksumMiddleware(stack)
	})
}

// OOS lifecycle XML types. These model exactly the elements the OOS PUT/GET
// Bucket Lifecycle API accepts and returns; anything the standard S3 SDK can
// express but OOS cannot is intentionally absent. The same structs are used for
// both marshaling the PUT body and unmarshaling the GET response, so the
// resource expand/flatten works against a single, OOS-accurate shape rather
// than the SDK's types.LifecycleRule (which lacks IsAccessTime/AtimeBase).
type oosLifecycleConfiguration struct {
	XMLName xml.Name           `xml:"LifecycleConfiguration"`
	XMLNS   string             `xml:"xmlns,attr"`
	Rules   []oosLifecycleRule `xml:"Rule"`
}

type oosLifecycleRule struct {
	ID         string               `xml:"ID,omitempty"`
	Prefix     string               `xml:"Prefix"`
	Status     string               `xml:"Status"`
	Expiration *oosLifecycleExpiry  `xml:"Expiration,omitempty"`
	Transition *oosLifecycleTransit `xml:"Transition,omitempty"`
	// AtimeBase is read-only: OOS returns it on GET for access-time-based rules
	// (the timestamp tracking was enabled). It is never sent on PUT.
	AtimeBase *int64 `xml:"AtimeBase,omitempty"`
}

// Days and Date are mutually exclusive within an Expiration/Transition; exactly
// one is emitted. Days is a pointer so that a zero-day value is still sent while
// Date stays empty (omitempty), and vice versa.
type oosLifecycleExpiry struct {
	Days *int32 `xml:"Days,omitempty"`
	Date string `xml:"Date,omitempty"`
}

type oosLifecycleTransit struct {
	Days         *int32 `xml:"Days,omitempty"`
	Date         string `xml:"Date,omitempty"`
	StorageClass string `xml:"StorageClass"`
	// IsAccessTime, when true, matches the object's last access time instead of
	// its last modified time. Only valid with Days. omitempty keeps it out of the
	// PUT body when false (OOS default).
	IsAccessTime *bool `xml:"IsAccessTime,omitempty"`
}

// marshalLifecycleBody renders the OOS PUT Bucket Lifecycle request body from
// already-OOS-shaped rules. The resource's expander is responsible for producing
// valid rules (e.g. at least one of Expiration/Transition); this function only
// serializes them, defaulting an empty rule set to an empty <LifecycleConfiguration>.
func marshalLifecycleBody(rules []oosLifecycleRule) ([]byte, error) {
	cfg := oosLifecycleConfiguration{
		XMLNS: oosXMLNamespace,
		Rules: rules,
	}

	body, err := xml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling OOS lifecycle body: %w", err)
	}

	return body, nil
}

// oosGetBucketLifecycle retrieves a bucket's lifecycle rules via the OOS
// GET /{bucket}?lifecycle request and parses the legacy <LifecycleConfiguration>
// response into OOS-native rules. Returns (nil, nil) when no configuration is set.
//
// It piggybacks on s3.GetBucketLifecycleConfiguration (same GET /{bucket}?lifecycle
// wiring), swapping the response deserializer for one that reads the OOS shape
// (with IsAccessTime/AtimeBase) rather than the SDK's standard-S3 model.
func oosGetBucketLifecycle(ctx context.Context, conn *s3.Client, bucket string) ([]oosLifecycleRule, error) {
	var rules []oosLifecycleRule

	_, err := conn.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSBucketLifecycleDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetBucketLifecycle response: %w", err)
						}

						// Swapping OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						// The replaced deserializer was responsible for populating
						// out.Result; a zero-value output satisfies the SDK's
						// top-level type assertion since we parse into our own struct.
						out.Result = &s3.GetBucketLifecycleConfigurationOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var cfg oosLifecycleConfiguration
						if err := xml.Unmarshal(data, &cfg); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetBucketLifecycle response: %w", err)
						}
						rules = cfg.Rules

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return rules, nil
}

// OOS logging XML types. OOS supports only TargetBucket / TargetPrefix under
// LoggingEnabled (no grants). The same structs serialize the PUT body and parse
// the GET response. An absent LoggingEnabled means logging is disabled.
type oosBucketLoggingStatus struct {
	XMLName        xml.Name           `xml:"BucketLoggingStatus"`
	XMLNS          string             `xml:"xmlns,attr,omitempty"`
	LoggingEnabled *oosLoggingEnabled `xml:"LoggingEnabled,omitempty"`
}

type oosLoggingEnabled struct {
	TargetBucket string `xml:"TargetBucket"`
	TargetPrefix string `xml:"TargetPrefix"`
}

// oosPutBucketLogging sends an OOS PUT Bucket Logging request. A nil enabled
// argument writes an empty <BucketLoggingStatus> body, which OOS interprets as
// "disable logging".
//
// As with lifecycle, the SDK marks PutBucketLogging RequireChecksum=true and
// emits x-amz-checksum-crc32; OOS only accepts the legacy Content-MD5 header, so
// we install our own body via replaceRequestBodyMiddleware and swap the checksum
// mechanism via withContentMD5. We piggyback on s3.PutBucketLogging purely for
// its PUT /{bucket}?logging wiring (signing, endpoint, retry).
func oosPutBucketLogging(ctx context.Context, conn *s3.Client, bucket string, enabled *oosLoggingEnabled) error {
	body, err := xml.Marshal(oosBucketLoggingStatus{
		XMLNS:          oosXMLNamespace,
		LoggingEnabled: enabled,
	})
	if err != nil {
		return fmt.Errorf("marshaling OOS logging body: %w", err)
	}

	_, err = conn.PutBucketLogging(ctx, &s3.PutBucketLoggingInput{
		Bucket:              aws.String(bucket),
		BucketLoggingStatus: &types.BucketLoggingStatus{},
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
		withContentMD5(o)
	})

	return err
}

// oosGetBucketLogging retrieves a bucket's logging configuration via the OOS
// GET /{bucket}?logging request. Returns nil when logging is disabled (no
// LoggingEnabled element in the response).
//
// It piggybacks on s3.GetBucketLogging, swapping the response deserializer for
// one that reads the OOS <BucketLoggingStatus> shape.
func oosGetBucketLogging(ctx context.Context, conn *s3.Client, bucket string) (*oosLoggingEnabled, error) {
	var result *oosLoggingEnabled

	_, err := conn.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSBucketLoggingDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetBucketLogging response: %w", err)
						}

						// Swapping OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						out.Result = &s3.GetBucketLoggingOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var status oosBucketLoggingStatus
						if err := xml.Unmarshal(data, &status); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetBucketLogging response: %w", err)
						}
						result = status.LoggingEnabled

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// oosPutBucketPolicy sends an OOS PUT Bucket Policy request. The body is the raw
// JSON policy, so unlike the XML operations there is no body to reshape; we only
// swap the checksum mechanism.
//
// The SDK marks PutBucketPolicy RequireChecksum=true and emits
// x-amz-checksum-crc32, which OOS rejects; withContentMD5 replaces it with the
// legacy Content-MD5 header OOS expects. We piggyback on s3.PutBucketPolicy for
// its PUT /{bucket}?policy wiring (signing, endpoint, retry).
func oosPutBucketPolicy(ctx context.Context, conn *s3.Client, bucket, policy string) error {
	_, err := conn.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(policy),
	}, func(o *s3.Options) {
		withContentMD5(o)
	})

	return err
}

// OOS object lock (compliance retention) XML types. OOS supports only Mode=COMPLIANCE and a
// DefaultRetention of either Days or Years. The same structs serialize the PUT
// body and parse the GET response.
type oosLockConfiguration struct {
	XMLName           xml.Name     `xml:"ObjectLockConfiguration"`
	XMLNS             string       `xml:"xmlns,attr,omitempty"`
	ObjectLockEnabled string       `xml:"ObjectLockEnabled"`
	Rule              *oosLockRule `xml:"Rule,omitempty"`
}

type oosLockRule struct {
	DefaultRetention *oosLockDefaultRetention `xml:"DefaultRetention,omitempty"`
}

// Days and Years are mutually exclusive; exactly one is emitted. Both are
// pointers so omitempty keeps the unused one out of the body.
type oosLockDefaultRetention struct {
	Mode  string `xml:"Mode"`
	Days  *int32 `xml:"Days,omitempty"`
	Years *int32 `xml:"Years,omitempty"`
}

// oosPutBucketObjectLock sends an OOS PUT Bucket Object Lock request. As with the
// other XML operations the SDK marks PutObjectLockConfiguration
// RequireChecksum=true and emits x-amz-checksum-crc32, which OOS rejects; we
// install our own body via replaceRequestBodyMiddleware and swap the checksum
// mechanism via withContentMD5. We piggyback on s3.PutObjectLockConfiguration for
// its PUT /{bucket}?object-lock wiring.
func oosPutBucketObjectLock(ctx context.Context, conn *s3.Client, bucket string, cfg oosLockConfiguration) error {
	cfg.XMLNS = oosXMLNamespace
	body, err := xml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling OOS object lock body: %w", err)
	}

	_, err = conn.PutObjectLockConfiguration(ctx, &s3.PutObjectLockConfigurationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
		withContentMD5(o)
	})

	return err
}

// oosGetBucketObjectLock retrieves a bucket's object lock configuration via the
// OOS GET /{bucket}?object-lock request. Returns nil when object lock is not
// configured.
//
// It piggybacks on s3.GetObjectLockConfiguration, swapping the response
// deserializer for one that reads the OOS <ObjectLockConfiguration> shape.
func oosGetBucketObjectLock(ctx context.Context, conn *s3.Client, bucket string) (*oosLockConfiguration, error) {
	var result *oosLockConfiguration

	_, err := conn.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSObjectLockDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetObjectLock response: %w", err)
						}

						// Swapping OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						out.Result = &s3.GetObjectLockConfigurationOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var cfg oosLockConfiguration
						if err := xml.Unmarshal(data, &cfg); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetObjectLock response: %w", err)
						}
						result = &cfg

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// OOS server-side encryption XML types. OOS supports only a single Rule whose
// ApplyServerSideEncryptionByDefault carries SSEAlgorithm=AES256 (no KMS key, no
// bucket key). The same structs serialize the PUT body and parse the GET
// response, so the resource expand/flatten works against an OOS-accurate shape
// rather than the SDK's types.ServerSideEncryptionConfiguration. See
// docs/oos-development/api/BucketServerSideEncryption.
type oosServerSideEncryptionConfiguration struct {
	XMLName xml.Name          `xml:"ServerSideEncryptionConfiguration"`
	XMLNS   string            `xml:"xmlns,attr,omitempty"`
	Rule    *oosSSEConfigRule `xml:"Rule,omitempty"`
}

type oosSSEConfigRule struct {
	ApplyServerSideEncryptionByDefault *oosSSEByDefault `xml:"ApplyServerSideEncryptionByDefault,omitempty"`
}

type oosSSEByDefault struct {
	SSEAlgorithm string `xml:"SSEAlgorithm"`
}

// oosPutBucketEncryption sends an OOS PUT Bucket Server Side Encryption request.
// OOS only accepts SSEAlgorithm=AES256.
//
// As with the other XML operations, the SDK marks PutBucketEncryption
// RequireChecksum=true and emits x-amz-checksum-crc32, which OOS rejects; we
// install our own body via replaceRequestBodyMiddleware and swap the checksum
// mechanism via withContentMD5. We piggyback on s3.PutBucketEncryption for its
// PUT /{bucket}?encryption wiring (signing, endpoint, retry).
func oosPutBucketEncryption(ctx context.Context, conn *s3.Client, bucket, algorithm string) error {
	body, err := xml.Marshal(oosServerSideEncryptionConfiguration{
		XMLNS: oosXMLNamespace,
		Rule: &oosSSEConfigRule{
			ApplyServerSideEncryptionByDefault: &oosSSEByDefault{
				SSEAlgorithm: algorithm,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshaling OOS encryption body: %w", err)
	}

	_, err = conn.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryption(algorithm),
					},
				},
			},
		},
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
		withContentMD5(o)
	})

	return err
}

// oosGetBucketEncryption retrieves a bucket's server-side encryption
// configuration via the OOS GET /{bucket}?encryption request. Returns
// (nil, nil) when no rule is configured (OOS returns an empty body in addition
// to the documented 404-NoSuchServerSideEncryptionRule, which surfaces as an
// error for the caller to classify).
//
// It piggybacks on s3.GetBucketEncryption, swapping the response deserializer
// for one that reads the OOS <ServerSideEncryptionConfiguration> shape.
func oosGetBucketEncryption(ctx context.Context, conn *s3.Client, bucket string) (*oosServerSideEncryptionConfiguration, error) {
	var result *oosServerSideEncryptionConfiguration

	_, err := conn.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSBucketEncryptionDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetBucketEncryption response: %w", err)
						}

						// Swapping OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						out.Result = &s3.GetBucketEncryptionOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var cfg oosServerSideEncryptionConfiguration
						if err := xml.Unmarshal(data, &cfg); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetBucketEncryption response: %w", err)
						}
						result = &cfg

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// oosDeleteBucketObjectLock removes an unenabled object lock configuration via the
// OOS-specific DELETE /{bucket}?object-lock request, which standard S3 does not
// provide. It piggybacks on s3.GetObjectLockConfiguration (also GET
// /{bucket}?object-lock) only to inherit the request wiring, rewrites the method
// to DELETE before signing, and swaps the response deserializer because OOS
// returns an empty success body.
func oosDeleteBucketObjectLock(ctx context.Context, conn *s3.Client, bucket string) error {
	_, err := conn.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions,
			setRequestMethodMiddleware("DELETE"),
			func(stack *middleware.Stack) error {
				// The response is empty; replace the deserializer so the SDK does not
				// try to parse a GetObjectLockConfiguration body.
				_, err := stack.Deserialize.Swap("OperationDeserializer",
					middleware.DeserializeMiddlewareFunc("OOSObjectLockDeleteDeserializer",
						func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
							out, metadata, err := next.HandleDeserialize(ctx, in)
							if err != nil {
								return out, metadata, err
							}

							resp, ok := out.RawResponse.(*smithyhttp.Response)
							if !ok {
								return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
							}
							defer resp.Body.Close()

							if resp.StatusCode < 200 || resp.StatusCode >= 300 {
								data, _ := io.ReadAll(resp.Body)
								return out, metadata, parseOOSError(resp.StatusCode, data)
							}

							out.Result = &s3.GetObjectLockConfigurationOutput{}
							return out, metadata, nil
						}),
				)
				return err
			},
		)
	})

	return err
}

// OOS soft delete XML types. OOS keeps deleted objects for a retention
// window of Days (1-15) before permanently removing them. See
// docs/api/BucketSoftDeleteConfiguration. The same struct serializes the PUT
// body and parses the GET response.
type oosSoftDeleteConfiguration struct {
	XMLName xml.Name `xml:"SoftDeleteConfiguration"`
	XMLNS   string   `xml:"xmlns,attr,omitempty"`
	Days    int32    `xml:"Days"`
}

// rewriteQueryMiddleware returns an API option that overwrites the request query
// string. OOS exposes operations under query keys (e.g. ?soft-delete) that the
// AWS SDK has no operation for, so these calls piggyback on the closest standard
// operation and swap its query here. It runs in the Build step (before signing)
// so SigV4 covers the rewritten query.
func rewriteQueryMiddleware(rawQuery string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("OOSRewriteQuery",
				func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						req.URL.RawQuery = rawQuery
					}
					return next.HandleBuild(ctx, in)
				}),
			middleware.After,
		)
	}
}

func setRequestMethodMiddleware(method string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("OOSSetRequestMethod",
				func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						req.Method = method
					}
					return next.HandleBuild(ctx, in)
				}),
			middleware.After,
		)
	}
}

// oosPutBucketSoftDelete sets a bucket's soft delete retention window via the OOS
// PUT /{bucket}?soft-delete request. days must be in [1, 15].
//
// There is no AWS SDK operation for ?soft-delete, so this piggybacks on
// s3.CreateBucket (PUT /{bucket}, no body, no flexible-checksum middleware),
// rewriting the query to soft-delete and installing the OOS XML body. Unlike the
// lifecycle/encryption operations, soft-delete signs with the standard
// x-amz-content-sha256 payload hash, so no Content-MD5 swap is needed.
func oosPutBucketSoftDelete(ctx context.Context, conn *s3.Client, bucket string, days int32) error {
	body, err := xml.Marshal(oosSoftDeleteConfiguration{
		XMLNS: oosXMLNamespace,
		Days:  days,
	})
	if err != nil {
		return fmt.Errorf("marshaling OOS soft delete body: %w", err)
	}

	_, err = conn.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions,
			rewriteQueryMiddleware("soft-delete"),
			replaceRequestBodyMiddleware(body),
		)
	})

	return err
}

// oosGetBucketSoftDelete retrieves a bucket's soft delete configuration via the
// OOS GET /{bucket}?soft-delete request. OOS always returns a configuration
// (a server-side default when none was set), so a nil result only occurs for an
// empty body.
//
// It piggybacks on s3.GetBucketLocation (GET /{bucket}?location), rewriting the
// query to soft-delete and swapping the response deserializer for one that reads
// the OOS <SoftDeleteConfiguration> shape.
func oosGetBucketSoftDelete(ctx context.Context, conn *s3.Client, bucket string) (*oosSoftDeleteConfiguration, error) {
	var result *oosSoftDeleteConfiguration

	_, err := conn.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, rewriteQueryMiddleware("soft-delete"))
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSBucketSoftDeleteDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS GetBucketSoftDelete response: %w", err)
						}

						// Swapping OperationDeserializer also removes the SDK's
						// restxml error decoding, so handle non-2xx here.
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						out.Result = &s3.GetBucketLocationOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							return out, metadata, nil
						}

						var cfg oosSoftDeleteConfiguration
						if err := xml.Unmarshal(data, &cfg); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS GetBucketSoftDelete response: %w", err)
						}
						result = &cfg

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type oosListObjectsSoftDeleteInput struct {
	bucket           string
	softDeleteAfter  string
	softDeleteBefore string
	marker           string
	maxKeys          int
	encodingType     string
}

type oosListObjectsSoftDeleteResult struct {
	XMLName                xml.Name                      `xml:"ListObjectsSoftDeleteResult"`
	Name                   string                        `xml:"Name"`
	SoftDeleteAfter        string                        `xml:"SoftDeleteAfter"`
	SoftDeleteBefore       string                        `xml:"SoftDeleteBefore"`
	SoftDeleteBeforeLegacy string                        `xml:"SoftDeleteBeforer"`
	Marker                 string                        `xml:"Marker"`
	NextMarker             string                        `xml:"NextMarker"`
	MaxKeys                int                           `xml:"MaxKeys"`
	EncodingType           string                        `xml:"EncodingType"`
	IsTruncated            bool                          `xml:"IsTruncated"`
	Contents               []oosListObjectsSoftDeleteObj `xml:"Contents"`
}

func (r *oosListObjectsSoftDeleteResult) softDeleteBefore() string {
	if r.SoftDeleteBefore != "" {
		return r.SoftDeleteBefore
	}

	return r.SoftDeleteBeforeLegacy
}

type oosListObjectsSoftDeleteObj struct {
	Key                string `xml:"Key"`
	LastModified       string `xml:"LastModified"`
	SoftDeleteAt       string `xml:"SoftDeleteAt"`
	SoftDeleteAtLegacy string `xml:"SoftDeletAt"`
	ETag               string `xml:"ETag"`
	Size               int64  `xml:"Size"`
	StorageClass       string `xml:"StorageClass"`
}

func (o oosListObjectsSoftDeleteObj) softDeleteAt() string {
	if o.SoftDeleteAt != "" {
		return o.SoftDeleteAt
	}

	return o.SoftDeleteAtLegacy
}

func listObjectsSoftDeleteRawQuery(in oosListObjectsSoftDeleteInput) string {
	parts := []string{"soft-delete-objects"}

	if in.softDeleteAfter != "" {
		parts = append(parts, "soft-delete-after="+url.QueryEscape(in.softDeleteAfter))
	}
	if in.softDeleteBefore != "" {
		parts = append(parts, "soft-delete-before="+url.QueryEscape(in.softDeleteBefore))
	}
	if in.maxKeys > 0 {
		parts = append(parts, "max-keys="+strconv.Itoa(in.maxKeys))
	}
	if in.marker != "" {
		parts = append(parts, "marker="+listObjectsSoftDeleteQueryMarker(in.marker, in.encodingType))
	}
	if in.encodingType != "" {
		parts = append(parts, "encoding-type="+url.QueryEscape(in.encodingType))
	}

	return strings.Join(parts, "&")
}

func listObjectsSoftDeleteQueryMarker(marker, encodingType string) string {
	if strings.EqualFold(encodingType, "url") {
		if decoded, err := url.QueryUnescape(marker); err == nil {
			marker = decoded
		}
	}

	return url.QueryEscape(marker)
}

// oosListObjectsSoftDelete retrieves one page of objects that are currently in
// a bucket's soft-delete retention window via GET /{bucket}?soft-delete-objects.
// There is no AWS SDK operation for this query, so it piggybacks on
// GetBucketLocation for the bucket endpoint wiring and swaps both the query and
// response deserializer.
func oosListObjectsSoftDelete(ctx context.Context, conn *s3.Client, in oosListObjectsSoftDeleteInput) (*oosListObjectsSoftDeleteResult, error) {
	var result *oosListObjectsSoftDeleteResult

	_, err := conn.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(in.bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, rewriteQueryMiddleware(listObjectsSoftDeleteRawQuery(in)))
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer",
				middleware.DeserializeMiddlewareFunc("OOSListObjectsSoftDeleteDeserializer",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, metadata, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, metadata, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok {
							return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
						}
						defer resp.Body.Close()

						data, err := io.ReadAll(resp.Body)
						if err != nil {
							return out, metadata, fmt.Errorf("reading OOS ListObjectsSoftDelete response: %w", err)
						}

						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							return out, metadata, parseOOSError(resp.StatusCode, data)
						}

						out.Result = &s3.GetBucketLocationOutput{}

						if len(bytes.TrimSpace(data)) == 0 {
							result = &oosListObjectsSoftDeleteResult{}
							return out, metadata, nil
						}

						var parsed oosListObjectsSoftDeleteResult
						if err := xml.Unmarshal(data, &parsed); err != nil {
							return out, metadata, fmt.Errorf("parsing OOS ListObjectsSoftDelete response: %w", err)
						}
						result = &parsed

						return out, metadata, nil
					}),
			)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &oosListObjectsSoftDeleteResult{}
	}

	return result, nil
}

// OOS recover object XML types. Recover Object restores objects that are still
// within the bucket's soft-delete retention window. It can recover an explicit
// object list or a soft-delete timestamp window, but the two modes are mutually
// exclusive.
type oosRecoverObjectsInput struct {
	bucket           string
	encodingType     string
	keys             []string
	softDeleteAfter  string
	softDeleteBefore string
}

type oosRecoverRequest struct {
	XMLName xml.Name           `xml:"Recover"`
	XMLNS   string             `xml:"xmlns,attr,omitempty"`
	Objects []oosRecoverObject `xml:"Object"`
}

type oosRecoverObject struct {
	Key string `json:"Key" xml:"Key"`
}

type oosRecoverObjectsResult struct {
	XMLName      xml.Name           `json:"-" xml:"RecoveredResult"`
	Recovered    []oosRecoverObject `json:"Recovered" xml:"Recovered"`
	RecoverError []oosRecoverError  `json:"Error" xml:"Error"`
}

type oosRecoverError struct {
	Key     string `json:"Key" xml:"Key"`
	Code    string `json:"Code" xml:"Code"`
	Message string `json:"Message" xml:"Message"`
}

func marshalRecoverObjectsBody(keys []string, encodingType string) ([]byte, error) {
	req := oosRecoverRequest{
		XMLNS:   oosXMLNamespace,
		Objects: make([]oosRecoverObject, 0, len(keys)),
	}

	for _, key := range keys {
		if strings.EqualFold(encodingType, "url") {
			key = url.QueryEscape(key)
		}
		req.Objects = append(req.Objects, oosRecoverObject{
			Key: key,
		})
	}

	body, err := xml.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling OOS RecoverObject body: %w", err)
	}

	return body, nil
}

func recoverObjectsRawQuery(in oosRecoverObjectsInput) string {
	parts := []string{"recover"}

	if in.softDeleteAfter != "" {
		parts = append(parts, "soft-delete-after="+url.QueryEscape(in.softDeleteAfter))
	}
	if in.softDeleteBefore != "" {
		parts = append(parts, "soft-delete-before="+url.QueryEscape(in.softDeleteBefore))
	}

	if in.encodingType != "" {
		parts = append(parts, "encoding-type="+url.QueryEscape(in.encodingType))
	}

	return strings.Join(parts, "&")
}

// oosRecoverObjects sends an OOS Recover Object request.
//
// There is no AWS SDK operation for ?recover, so this piggybacks on
// s3.CreateBucket for its bucket endpoint wiring, then changes the request into
// POST /{bucket}?recover, optionally installs the OOS XML body, and swaps the
// response deserializer to parse <RecoveredResult>. The request still runs
// through endpoint resolution, SigV4 signing, region injection, retry, and VCR.
func oosRecoverObjects(ctx context.Context, conn *s3.Client, in oosRecoverObjectsInput) (*oosRecoverObjectsResult, error) {
	var result *oosRecoverObjectsResult
	var body []byte

	if len(in.keys) > 0 {
		var err error
		body, err = marshalRecoverObjectsBody(in.keys, in.encodingType)
		if err != nil {
			return nil, err
		}
	}

	_, err := conn.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(in.bucket),
	}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions,
			setRequestMethodMiddleware("POST"),
			rewriteQueryMiddleware(recoverObjectsRawQuery(in)),
			recoverObjectsDeserializer(&result),
		)
		if len(body) > 0 {
			o.APIOptions = append(o.APIOptions, replaceRequestBodyMiddleware(body))
		}
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &oosRecoverObjectsResult{}
	}

	return result, nil
}

func recoverObjectsDeserializer(result **oosRecoverObjectsResult) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		_, err := stack.Deserialize.Swap("OperationDeserializer",
			middleware.DeserializeMiddlewareFunc("OOSRecoverObjectsDeserializer",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, metadata, err := next.HandleDeserialize(ctx, in)
					if err != nil {
						return out, metadata, err
					}

					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok {
						return out, metadata, fmt.Errorf("unexpected transport type %T", out.RawResponse)
					}
					defer resp.Body.Close()

					data, err := io.ReadAll(resp.Body)
					if err != nil {
						return out, metadata, fmt.Errorf("reading OOS RecoverObject response: %w", err)
					}

					if resp.StatusCode < 200 || resp.StatusCode >= 300 {
						return out, metadata, parseOOSError(resp.StatusCode, data)
					}

					out.Result = &s3.CreateBucketOutput{}

					parsed, err := parseRecoverObjectsResult(data)
					if err != nil {
						return out, metadata, err
					}
					*result = parsed

					return out, metadata, nil
				}),
		)
		return err
	}
}

func parseRecoverObjectsResult(data []byte) (*oosRecoverObjectsResult, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &oosRecoverObjectsResult{}, nil
	}

	var result oosRecoverObjectsResult
	if err := xml.Unmarshal(data, &result); err == nil {
		return &result, nil
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing OOS RecoverObject response: %w", err)
	}

	return &result, nil
}

// OOS object data-location support.
//
// OOS adds the request header x-ctyun-data-location to PUT Object / PUT Object -
// Copy, controlling where the object's data is physically placed. Its value is a
// small key/value policy string the AWS SDK has no field for, so the resource
// models it as a nested block and serializes it here. On GET/HEAD OOS instead
// returns the already-resolved single location in the x-ctyun-data-location
// response header (a bare location name, NOT the policy string), plus the index
// location in x-ctyun-metadata-location; those are surfaced as read-only
// attributes via captureOOSObjectHeadersMiddleware.
const (
	oosDataLocationTypeLocal     = "Local"
	oosDataLocationTypeSpecified = "Specified"

	oosHeaderContentMD5       = "Content-MD5"
	oosHeaderDataLocation     = "x-ctyun-data-location"
	oosHeaderMetadataLocation = "x-ctyun-metadata-location"
)

// oosObjectDataLocation is the expanded form of the data_location nested block.
type oosObjectDataLocation struct {
	Type             string
	Locations        []string
	ScheduleStrategy string
}

// formatDataLocationHeader renders the x-ctyun-data-location request header value
// from a data_location block, e.g. "type=Local,scheduleStrategy=Allowed" or
// "type=Specified,location=ChengDu,WuHan,scheduleStrategy=NotAllowed". Returns ""
// when no data location is configured, so the caller omits the header entirely.
func formatDataLocationHeader(in *oosObjectDataLocation) string {
	if in == nil || in.Type == "" {
		return ""
	}

	parts := []string{"type=" + in.Type}
	if in.Type == oosDataLocationTypeSpecified && len(in.Locations) > 0 {
		parts = append(parts, "location="+strings.Join(in.Locations, ","))
	}
	if in.ScheduleStrategy != "" {
		parts = append(parts, "scheduleStrategy="+in.ScheduleStrategy)
	}

	return strings.Join(parts, ",")
}

// setDataLocationHeaderMiddleware returns an API option that sets the
// x-ctyun-data-location request header. It runs in the Build step (before
// signing) so SigV4 covers the header. value must be non-empty.
func setDataLocationHeaderMiddleware(value string) func(*middleware.Stack) error {
	return setRequestHeaderMiddleware("OOSSetDataLocation", oosHeaderDataLocation, value)
}

// setRequestHeaderMiddleware returns an API option that sets an arbitrary
// request header. It runs in the Build step (before signing) so SigV4 covers the
// header. id must be unique within the middleware stack.
func setRequestHeaderMiddleware(id, header, value string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc(id,
				func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						if header == strings.ToLower(header) {
							req.Header[header] = []string{value}
						} else {
							req.Header.Set(header, value)
						}
					}
					return next.HandleBuild(ctx, in)
				}),
			middleware.After,
		)
	}
}

// setLowercaseMetadataHeadersMiddleware writes user metadata request headers as
// literal lowercase x-amz-meta-* names. OOS requires these header names to stay
// lowercase; using http.Header.Set would canonicalize them to X-Amz-Meta-*,
// which OOS ignores.
func setLowercaseMetadataHeadersMiddleware(metadata map[string]string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("OOSSetLowercaseMetadataHeaders",
				func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						for k, v := range metadata {
							header := "x-amz-meta-" + strings.ToLower(k)
							req.Header[header] = []string{v}
						}
					}
					return next.HandleBuild(ctx, in)
				}),
			middleware.After,
		)
	}
}

// oosObjectHeaders holds the OOS-specific response headers returned by GET/HEAD
// Object that the SDK's HeadObjectOutput does not model.
type oosObjectHeaders struct {
	contentMD5       string // Content-MD5: the object's Base64-encoded MD5 digest
	metadataLocation string // x-ctyun-metadata-location: the index location
	dataLocation     string // x-ctyun-data-location: the resolved data location
}

// captureOOSObjectHeadersMiddleware returns an API option that reads the OOS
// x-ctyun-* response headers into out. It is added (not swapped) after the
// standard OperationDeserializer, so the SDK still parses the normal
// HeadObject/GetObject response; this middleware only inspects headers and never
// touches the body.
func captureOOSObjectHeadersMiddleware(out *oosObjectHeaders) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Add(
			middleware.DeserializeMiddlewareFunc("OOSCaptureObjectHeaders",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					o, metadata, err := next.HandleDeserialize(ctx, in)
					if err != nil {
						return o, metadata, err
					}

					if resp, ok := o.RawResponse.(*smithyhttp.Response); ok {
						out.contentMD5 = resp.Header.Get(oosHeaderContentMD5)
						out.metadataLocation = resp.Header.Get(oosHeaderMetadataLocation)
						out.dataLocation = resp.Header.Get(oosHeaderDataLocation)
					}

					return o, metadata, err
				}),
			middleware.Before,
		)
	}
}
