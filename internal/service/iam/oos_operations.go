// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	smithy "github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
)

const (
	oosIAMPolicyDocumentMetadataKey = "oos_policy_document"
	oosIAMPolicyScopeMetadataKey    = "oos_policy_scope"
	oosIAMPolicyScopesMetadataKey   = "oos_policy_scopes"
	oosIAMGetGroupMetadataKey       = "oos_get_group"
	oosIAMUserIPLastUsedMetadataKey = "oos_user_ip_last_used"
	oosIAMListGroupsMetadataKey     = "oos_list_groups"
	oosIAMListUsersMetadataKey      = "oos_list_users"
	oosIAMAttachedPoliciesKey       = "oos_attached_policies"
	oosIAMUnknownErrorCode          = "UnknownError"
	oosIAMInvalidHTTPResponseCode   = "InvalidHTTPResponse"
	oosIAMDiagnosticBodyLimit       = 1024

	// OOS ListSAMLProviders supports Marker/IsTruncated, but the AWS SDK model does not expose them; parse the pagination metadata from the response body.
	oosIAMListSAMLProvidersPageMetaKey = "oos_list_saml_providers_page"
)

// withOOSIAMQueryBody replaces the SDK operation payload with an OOS IAM Query form body.
func withOOSIAMQueryBody(action string, values url.Values) func(*iam.Options) {
	queryValues := cloneURLValues(values)
	queryValues.Set("Action", action)
	queryValues.Set("Version", iam.ServiceAPIVersion)

	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			if err := stack.Serialize.Add(
				middleware.SerializeMiddlewareFunc("OOSIAMQuerySerialize", func(ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler) (middleware.SerializeOutput, middleware.Metadata, error) {
					req, ok := in.Request.(*smithyhttp.Request)
					if !ok {
						return middleware.SerializeOutput{}, middleware.Metadata{}, fmt.Errorf("unexpected request type %T", in.Request)
					}

					req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
					req, err := req.SetStream(bytes.NewReader([]byte(queryValues.Encode())))
					if err != nil {
						return middleware.SerializeOutput{}, middleware.Metadata{}, err
					}
					in.Request = req

					return next.HandleSerialize(ctx, in)
				}),
				middleware.After,
			); err != nil {
				return err
			}
			return nil
		})
	}
}

// withOOSIAMQueryResponse captures an OOS IAM Query response for resource-specific XML parsing.
func withOOSIAMQueryResponse(result any, responseBody *[]byte) func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			_, err := stack.Deserialize.Swap("OperationDeserializer", middleware.DeserializeMiddlewareFunc("OOSIAMQueryResponse", func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
				out, md, err := next.HandleDeserialize(ctx, in)
				if err != nil {
					return out, md, err
				}

				resp, ok := out.RawResponse.(*smithyhttp.Response)
				if !ok || resp == nil || resp.Body == nil {
					return out, md, fmt.Errorf("unexpected response type %T", out.RawResponse)
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return out, md, err
				}

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					return out, md, parseOOSIAMErrorBody(resp.StatusCode, b)
				}

				*responseBody = b
				out.Result = result

				return out, md, nil
			}))
			return err
		})
	}
}

func cloneURLValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for k, v := range values {
		clone[k] = append([]string(nil), v...)
	}

	return clone
}

type oosGetPolicyResponse struct {
	GetPolicyResult *oosGetPolicyResult `xml:"GetPolicyResult"`
}

type oosGetPolicyResult struct {
	Policy *oosGetPolicyResponsePolicy `xml:"Policy"`
}

type oosGetPolicyResponsePolicy struct {
	Document string `xml:"Document"`
	Scope    string `xml:"Scope"`
}

type oosGetGroupResponse struct {
	GetGroupResult *oosGetGroupResult `xml:"GetGroupResult"`
}

type oosGetGroupResult struct {
	MemberUsers []oosGetGroupResponseUser `xml:"Users>member"`
	UserUsers   []oosGetGroupResponseUser `xml:"Users>User"`
}

type oosGetGroupResponseUser struct {
	UserName string `xml:"UserName"`
	JoinDate string `xml:"JoinDate"`
}

type oosGetGroupMetadata struct {
	JoinDate string
}

type oosListPoliciesResponse struct {
	ListPoliciesResult *oosListPoliciesResult `xml:"ListPoliciesResult"`
}

type oosListPoliciesResult struct {
	MemberPolicies []oosListPoliciesResponsePolicy `xml:"Policies>member"`
	PolicyPolicies []oosListPoliciesResponsePolicy `xml:"Policies>Policy"`
}

type oosListPoliciesResponsePolicy struct {
	PolicyId string `xml:"PolicyId"`
	Scope    string `xml:"Scope"`
}

type oosListAttachedPoliciesResponse struct {
	ListAttachedGroupPoliciesResult *oosListAttachedPoliciesResult `xml:"ListAttachedGroupPoliciesResult"`
	ListAttachedRolePoliciesResult  *oosListAttachedPoliciesResult `xml:"ListAttachedRolePoliciesResult"`
	ListAttachedUserPoliciesResult  *oosListAttachedPoliciesResult `xml:"ListAttachedUserPoliciesResult"`
}

type oosListAttachedPoliciesResult struct {
	MemberPolicies []oosListAttachedPoliciesResponsePolicy `xml:"AttachedPolicies>member"`
	PolicyPolicies []oosListAttachedPoliciesResponsePolicy `xml:"AttachedPolicies>Policy"`
}

type oosListAttachedPoliciesResponsePolicy struct {
	Description string `xml:"Description"`
	PolicyArn   string `xml:"PolicyArn"`
	Scope       string `xml:"Scope"`
}

type oosAttachedPolicyMetadata struct {
	Description string
	Scope       string
}

type oosListGroupsResponse struct {
	ListGroupsResult *oosListGroupsResult `xml:"ListGroupsResult"`
}

type oosListGroupsResult struct {
	MemberGroups []oosListGroupsResponseGroup `xml:"Groups>member"`
	GroupGroups  []oosListGroupsResponseGroup `xml:"Groups>Group"`
}

type oosListGroupsResponseGroup struct {
	GroupName string `xml:"GroupName"`
	Policies  int    `xml:"Policies"`
	Users     int    `xml:"Users"`
}

type oosListGroupsMetadata struct {
	Policies int
	Users    int
}

type oosGetUserResponse struct {
	GetUserResult *oosGetUserResult `xml:"GetUserResult"`
}

type oosGetUserResult struct {
	User *oosGetUserResponseUser `xml:"User"`
}

type oosGetUserResponseUser struct {
	IPLastUsed string `xml:"IPLastUsed"`
}

type oosListUsersResponse struct {
	ListUsersResult *oosListUsersResult `xml:"ListUsersResult"`
}

type oosListUsersResult struct {
	MemberUsers []oosListUsersResponseUser `xml:"Users>member"`
	UserUsers   []oosListUsersResponseUser `xml:"Users>User"`
}

type oosListUsersResponseUser struct {
	UserName           string `xml:"UserName"`
	MFADeviceCount     int    `xml:"MFADeviceCount"`
	AccessKeyCount     int    `xml:"AccessKeyCount"`
	PasswordCreateDate string `xml:"PasswordCreateDate"`
}

type oosListUsersMetadata struct {
	MFADeviceCount     int
	AccessKeyCount     int
	PasswordCreateDate string
}

type oosGetPolicyMetadata struct {
	Document string
	Scope    string
}

func parseOOSIAMErrorBody(statusCode int, b []byte) error {
	var parsed struct {
		Code      string `xml:"Code"`
		Message   string `xml:"Message"`
		Resource  string `xml:"Resource"`
		RequestID string `xml:"RequestId"`
	}

	trimmed := strings.TrimSpace(string(b))
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return newOOSIAMAPIError(statusCode, oosIAMUnknownErrorCode, trimmed)
	}

	code := strings.TrimSpace(parsed.Code)
	if code == "" {
		code = oosIAMUnknownErrorCode
	}

	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = trimmed
	}
	if message == "" {
		message = code
	}

	return newOOSIAMAPIError(statusCode, code, message)
}

func oosIAMErrorRequestID(b []byte) string {
	var parsed struct {
		RequestID string `xml:"RequestId"`
	}
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return ""
	}

	return strings.TrimSpace(parsed.RequestID)
}

func newOOSIAMAPIError(statusCode int, code, message string) error {
	apiErr := &smithy.GenericAPIError{Code: code, Message: message}
	if statusCode == http.StatusNotFound || strings.EqualFold(code, "NoSuchEntity") || strings.EqualFold(code, "NoSuchEntityException") {
		return &retry.NotFoundError{LastError: apiErr}
	}

	return apiErr
}

func parseIAMGetGroupMetadata(resp *http.Response) (map[string]oosGetGroupMetadata, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return nil, err
	}

	var parsed oosGetGroupResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	if parsed.GetGroupResult == nil {
		return nil, nil
	}

	users := parsed.GetGroupResult.MemberUsers
	if len(users) == 0 {
		users = parsed.GetGroupResult.UserUsers
	}

	output := make(map[string]oosGetGroupMetadata, len(users))
	for _, user := range users {
		if user.UserName == "" {
			continue
		}
		output[user.UserName] = oosGetGroupMetadata{
			JoinDate: user.JoinDate,
		}
	}

	return output, nil
}

func parseIAMGetUserIPLastUsed(resp *http.Response) (string, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return "", err
	}

	var parsed oosGetUserResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return "", err
	}

	if parsed.GetUserResult == nil || parsed.GetUserResult.User == nil {
		return "", nil
	}

	return parsed.GetUserResult.User.IPLastUsed, nil
}

func parseIAMGetPolicyMetadata(resp *http.Response) (oosGetPolicyMetadata, error) {
	if resp.Body == nil {
		return oosGetPolicyMetadata{}, nil
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return oosGetPolicyMetadata{}, err
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(b))

	if len(bytes.TrimSpace(b)) == 0 {
		return oosGetPolicyMetadata{}, nil
	}

	var parsed oosGetPolicyResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return oosGetPolicyMetadata{}, err
	}

	if parsed.GetPolicyResult == nil || parsed.GetPolicyResult.Policy == nil {
		return oosGetPolicyMetadata{}, nil
	}

	return oosGetPolicyMetadata{
		Document: parsed.GetPolicyResult.Policy.Document,
		Scope:    parsed.GetPolicyResult.Policy.Scope,
	}, nil
}

func parseIAMListUsersMetadata(resp *http.Response) (map[string]oosListUsersMetadata, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return nil, err
	}

	var parsed oosListUsersResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	if parsed.ListUsersResult == nil {
		return nil, nil
	}

	users := parsed.ListUsersResult.MemberUsers
	if len(users) == 0 {
		users = parsed.ListUsersResult.UserUsers
	}

	output := make(map[string]oosListUsersMetadata, len(users))
	for _, user := range users {
		if user.UserName == "" {
			continue
		}
		output[user.UserName] = oosListUsersMetadata{
			MFADeviceCount:     user.MFADeviceCount,
			AccessKeyCount:     user.AccessKeyCount,
			PasswordCreateDate: user.PasswordCreateDate,
		}
	}

	return output, nil
}

func parseIAMListGroupsMetadata(resp *http.Response) (map[string]oosListGroupsMetadata, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return nil, err
	}

	var parsed oosListGroupsResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	if parsed.ListGroupsResult == nil {
		return nil, nil
	}

	groups := parsed.ListGroupsResult.MemberGroups
	if len(groups) == 0 {
		groups = parsed.ListGroupsResult.GroupGroups
	}

	output := make(map[string]oosListGroupsMetadata, len(groups))
	for _, group := range groups {
		if group.GroupName == "" {
			continue
		}
		output[group.GroupName] = oosListGroupsMetadata{
			Policies: group.Policies,
			Users:    group.Users,
		}
	}

	return output, nil
}

func parseIAMListPolicyScopes(resp *http.Response) (map[string]string, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return nil, err
	}

	var parsed oosListPoliciesResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	if parsed.ListPoliciesResult == nil {
		return nil, nil
	}

	policies := parsed.ListPoliciesResult.MemberPolicies
	if len(policies) == 0 {
		policies = parsed.ListPoliciesResult.PolicyPolicies
	}

	scopes := make(map[string]string, len(policies))
	for _, policy := range policies {
		if policy.PolicyId != "" && policy.Scope != "" {
			scopes[policy.PolicyId] = policy.Scope
		}
	}

	return scopes, nil
}

func parseIAMListAttachedPoliciesMetadata(resp *http.Response) (map[string]oosAttachedPolicyMetadata, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return nil, err
	}

	var parsed oosListAttachedPoliciesResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}

	var result *oosListAttachedPoliciesResult
	switch {
	case parsed.ListAttachedGroupPoliciesResult != nil:
		result = parsed.ListAttachedGroupPoliciesResult
	case parsed.ListAttachedRolePoliciesResult != nil:
		result = parsed.ListAttachedRolePoliciesResult
	case parsed.ListAttachedUserPoliciesResult != nil:
		result = parsed.ListAttachedUserPoliciesResult
	default:
		return nil, nil
	}

	policies := result.MemberPolicies
	if len(policies) == 0 {
		policies = result.PolicyPolicies
	}

	output := make(map[string]oosAttachedPolicyMetadata, len(policies))
	for _, policy := range policies {
		if policy.PolicyArn == "" {
			continue
		}
		output[policy.PolicyArn] = oosAttachedPolicyMetadata{
			Description: policy.Description,
			Scope:       policy.Scope,
		}
	}

	return output, nil
}

func readAndRestoreResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(b))

	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}

	return b, nil
}

type oosListSAMLProvidersPageMeta struct {
	IsTruncated bool
	Marker      string
}

type oosListSAMLProvidersResponse struct {
	ListSAMLProvidersResult *struct {
		IsTruncated bool   `xml:"IsTruncated"`
		Marker      string `xml:"Marker"`
	} `xml:"ListSAMLProvidersResult"`
}

func parseOOSListSAMLProvidersPageMeta(resp *http.Response) (oosListSAMLProvidersPageMeta, error) {
	b, err := readAndRestoreResponseBody(resp)
	if err != nil || len(b) == 0 {
		return oosListSAMLProvidersPageMeta{}, err
	}

	var parsed oosListSAMLProvidersResponse
	if err := xml.Unmarshal(b, &parsed); err != nil {
		return oosListSAMLProvidersPageMeta{}, err
	}
	if parsed.ListSAMLProvidersResult == nil {
		return oosListSAMLProvidersPageMeta{}, nil
	}

	return oosListSAMLProvidersPageMeta{
		IsTruncated: parsed.ListSAMLProvidersResult.IsTruncated,
		Marker:      strings.TrimSpace(parsed.ListSAMLProvidersResult.Marker),
	}, nil
}

// withOOSIAMListSAMLProvidersPageMeta parses IsTruncated/Marker from the OOS response body after the SDK deserializes
// the SAMLProviderList (the SDK ListSAMLProvidersOutput does not include these two fields).
func withOOSIAMListSAMLProvidersPageMeta(pageMeta *oosListSAMLProvidersPageMeta) func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Deserialize.Insert(
				middleware.DeserializeMiddlewareFunc(
					"OOSIAMListSAMLProvidersPageMeta",
					func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						out, md, err := next.HandleDeserialize(ctx, in)
						if err != nil {
							return out, md, err
						}

						resp, ok := out.RawResponse.(*smithyhttp.Response)
						if !ok || resp == nil || resp.Response == nil {
							return out, md, err
						}

						parsed, parseErr := parseOOSListSAMLProvidersPageMeta(resp.Response)
						if parseErr != nil {
							return out, md, parseErr
						}
						if pageMeta != nil {
							*pageMeta = parsed
						}
						md.Set(oosIAMListSAMLProvidersPageMetaKey, parsed)

						return out, md, err
					},
				),
				"OperationDeserializer",
				middleware.After,
			)
		})
	}
}

func handleOOSIAMErrorResponse(resp *smithyhttp.Response, md *middleware.Metadata, deserializeErr error) error {
	if resp == nil || resp.Response == nil {
		return nil
	}

	data, err := readAndRestoreResponseBody(resp.Response)
	if err != nil {
		return err
	}

	if requestID := oosIAMErrorRequestID(data); requestID != "" {
		awsmiddleware.SetRequestIDMetadata(md, requestID)
	}
	if resp.StatusCode == 0 {
		return newOOSIAMInvalidHTTPResponseError(resp.Response, data, deserializeErr)
	}

	return parseOOSIAMErrorBody(resp.StatusCode, data)
}

func newOOSIAMInvalidHTTPResponseError(resp *http.Response, body []byte, cause error) error {
	endpoint := ""
	if resp.Request != nil && resp.Request.URL != nil {
		endpoint = resp.Request.URL.Host
	}

	message := fmt.Sprintf(
		"IAM endpoint returned an invalid HTTP response (endpoint=%q, status=%q, headers=%s, body=%q)",
		endpoint,
		resp.Status,
		oosIAMDiagnosticHeaders(resp.Header),
		oosIAMTruncateDiagnosticBody(body),
	)
	if cause != nil {
		message += fmt.Sprintf(", cause=%q", oosIAMTruncateDiagnosticBody([]byte(cause.Error())))
	}

	return newOOSIAMAPIError(resp.StatusCode, oosIAMInvalidHTTPResponseCode, message)
}

func oosIAMDiagnosticHeaders(headers http.Header) string {
	keys := []string{
		"Content-Type",
		"Content-Length",
		"Server",
		"Via",
		"X-Request-Id",
		"X-Amzn-Requestid",
		"X-Correlation-Id",
		"X-Envoy-Upstream-Service-Time",
	}

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.Join(headers.Values(key), ","); value != "" {
			values = append(values, fmt.Sprintf("%s=%q", key, value))
		}
	}

	return "{" + strings.Join(values, ", ") + "}"
}

func oosIAMTruncateDiagnosticBody(body []byte) string {
	if len(body) <= oosIAMDiagnosticBodyLimit {
		return strings.TrimSpace(string(body))
	}

	return strings.TrimSpace(string(body[:oosIAMDiagnosticBodyLimit])) + "...[truncated]"
}

func addOOSIAMErrorResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"OOSIAMErrorResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					if resp.StatusCode < 200 || resp.StatusCode >= 300 {
						return out, md, handleOOSIAMErrorResponse(resp, &md, err)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func withOOSIAMGetGroupResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMGetGroupResponseMiddleware())
	}
}

func withOOSIAMGetPolicyResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMGetPolicyResponseMiddleware())
	}
}

func withOOSIAMGetUserResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMGetUserResponseMiddleware())
	}
}

func withOOSIAMListPoliciesResponse(policyName string) func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions,
			addOOSIAMListPoliciesPolicyNameMiddleware(policyName),
			addOOSIAMListPoliciesResponseMiddleware(),
		)
	}
}

func withOOSIAMListAttachedPoliciesResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMListAttachedPoliciesResponseMiddleware())
	}
}

func withOOSIAMListGroupsResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMListGroupsResponseMiddleware())
	}
}

func withOOSIAMListUsersResponse() func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, addOOSIAMListUsersResponseMiddleware())
	}
}

func addOOSIAMListPoliciesPolicyNameMiddleware(policyName string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"AddOOSIAMListPoliciesPolicyName",
				func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					if middleware.GetOperationName(ctx) != "ListPolicies" || policyName == "" {
						return next.HandleFinalize(ctx, in)
					}

					req, ok := in.Request.(*smithyhttp.Request)
					if !ok || req == nil {
						return next.HandleFinalize(ctx, in)
					}

					stream := req.GetStream()
					if stream == nil {
						return next.HandleFinalize(ctx, in)
					}

					body, err := io.ReadAll(stream)
					if err != nil {
						return middleware.FinalizeOutput{}, middleware.Metadata{}, err
					}

					values, err := url.ParseQuery(string(body))
					if err != nil {
						return middleware.FinalizeOutput{}, middleware.Metadata{}, err
					}

					values.Set("PolicyName", policyName)
					encoded := values.Encode()

					updatedReq, err := req.SetStream(bytes.NewReader([]byte(encoded)))
					if err != nil {
						return middleware.FinalizeOutput{}, middleware.Metadata{}, err
					}
					updatedReq.ContentLength = int64(len(encoded))
					updatedReq.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
					in.Request = updatedReq

					return next.HandleFinalize(ctx, in)
				},
			),
			middleware.Before,
		)
	}
}

func withOOSIAMDeleteUserForceDestroy(forceDestroy bool) func(*iam.Options) {
	return func(o *iam.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Finalize.Add(
				middleware.FinalizeMiddlewareFunc(
					"AddOOSIAMDeleteUserForceDestroy",
					func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
						if middleware.GetOperationName(ctx) != "DeleteUser" {
							return next.HandleFinalize(ctx, in)
						}

						req, ok := in.Request.(*smithyhttp.Request)
						if !ok || req == nil {
							return next.HandleFinalize(ctx, in)
						}

						stream := req.GetStream()
						if stream == nil {
							return next.HandleFinalize(ctx, in)
						}

						body, err := io.ReadAll(stream)
						if err != nil {
							return middleware.FinalizeOutput{}, middleware.Metadata{}, err
						}

						values, err := url.ParseQuery(string(body))
						if err != nil {
							return middleware.FinalizeOutput{}, middleware.Metadata{}, err
						}

						values.Set("ForceDestroy", strconv.FormatBool(forceDestroy))
						encoded := values.Encode()
						updatedReq, err := req.SetStream(bytes.NewReader([]byte(encoded)))
						if err != nil {
							return middleware.FinalizeOutput{}, middleware.Metadata{}, err
						}
						updatedReq.ContentLength = int64(len(encoded))
						updatedReq.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
						in.Request = updatedReq

						return next.HandleFinalize(ctx, in)
					},
				),
				middleware.Before,
			)
		})
	}
}

func addOOSIAMGetGroupResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMGetGroupResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					users, parseErr := parseIAMGetGroupMetadata(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if len(users) > 0 {
						md.Set(oosIAMGetGroupMetadataKey, users)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMGetPolicyResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMGetPolicyResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					policyMetadata, parseErr := parseIAMGetPolicyMetadata(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if policyMetadata.Document != "" {
						md.Set(oosIAMPolicyDocumentMetadataKey, policyMetadata.Document)
					}
					if policyMetadata.Scope != "" {
						md.Set(oosIAMPolicyScopeMetadataKey, policyMetadata.Scope)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMGetUserResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMGetUserResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					ipLastUsed, parseErr := parseIAMGetUserIPLastUsed(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if ipLastUsed != "" {
						md.Set(oosIAMUserIPLastUsedMetadataKey, ipLastUsed)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMListPoliciesResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMListPoliciesResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					scopes, parseErr := parseIAMListPolicyScopes(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if len(scopes) > 0 {
						md.Set(oosIAMPolicyScopesMetadataKey, scopes)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMListAttachedPoliciesResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMListAttachedPoliciesResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					switch middleware.GetOperationName(ctx) {
					case "ListAttachedGroupPolicies", "ListAttachedRolePolicies", "ListAttachedUserPolicies":
					default:
						return out, md, err
					}

					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					policies, parseErr := parseIAMListAttachedPoliciesMetadata(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if len(policies) > 0 {
						md.Set(oosIAMAttachedPoliciesKey, policies)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMListGroupsResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMListGroupsResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					groups, parseErr := parseIAMListGroupsMetadata(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if len(groups) > 0 {
						md.Set(oosIAMListGroupsMetadataKey, groups)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}

func addOOSIAMListUsersResponseMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Insert(
			middleware.DeserializeMiddlewareFunc(
				"IAMListUsersResponse",
				func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleDeserialize(ctx, in)
					resp, ok := out.RawResponse.(*smithyhttp.Response)
					if !ok || resp == nil || resp.Response == nil {
						return out, md, err
					}

					users, parseErr := parseIAMListUsersMetadata(resp.Response)
					if parseErr != nil {
						return out, md, parseErr
					}
					if len(users) > 0 {
						md.Set(oosIAMListUsersMetadataKey, users)
					}

					return out, md, err
				},
			),
			"OperationDeserializer",
			middleware.After,
		)
	}
}
