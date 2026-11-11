// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

// Error code constants missing from AWS Go SDK:
// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#pkg-constants

const (
	errCodeAccessDenied                         = "AccessDenied"
	errCodeBucketAlreadyExists                  = "BucketAlreadyExists"
	errCodeBucketAlreadyOwnedByYou              = "BucketAlreadyOwnedByYou"
	errCodeBucketNotEmpty                       = "BucketNotEmpty"
	errCodeIllegalLocationConstraintException   = "IllegalLocationConstraintException"
	errCodeInvalidArgument                      = "InvalidArgument"
	errCodeInvalidBucketState                   = "InvalidBucketState"
	errCodeInvalidRequest                       = "InvalidRequest"
	errCodeMalformedPolicy                      = "MalformedPolicy"
	errCodeMetadataConfigurationNotFound        = "MetadataConfigurationNotFound"
	errCodeMethodNotAllowed                     = "MethodNotAllowed"
	errCodeNoSuchBucket                         = "NoSuchBucket"
	errCodeNoSuchBucketPolicy                   = "NoSuchBucketPolicy"
	errCodeNotSuchBucketPolicy                  = "NotSuchBucketPolicy"
	errCodeNoSuchConfiguration                  = "NoSuchConfiguration"
	errCodeNoSuchCORSConfiguration              = "NoSuchCORSConfiguration"
	errCodeNoSuchLifecycleConfiguration         = "NoSuchLifecycleConfiguration"
	errCodeNoSuchKey                            = "NoSuchKey"
	errCodeNoSuchPublicAccessBlockConfiguration = "NoSuchPublicAccessBlockConfiguration"
	// errCodeNoSuchServerSideEncryptionRule is the OOS-specific 404 code returned by
	// GET /{bucket}?encryption when the bucket has no encryption rule configured.
	errCodeNoSuchServerSideEncryptionRule = "NoSuchServerSideEncryptionRule"
	errCodeNoSuchWebsiteConfiguration     = "NoSuchWebsiteConfiguration"
	errCodeNotImplemented                 = "NotImplemented"
	// errCodeObjectLockConfigurationNotFound should be used with tfawserr.ErrCodeContains, not tfawserr.ErrCodeEquals.
	// Reference: https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/pull/26317.
	errCodeObjectLockConfigurationNotFound           = "ObjectLockConfigurationNotFound"
	errCodeObjectLockConfigurationNotFoundError      = "ObjectLockConfigurationNotFoundError"
	errCodeOwnershipControlsNotFoundError            = "OwnershipControlsNotFoundError"
	errCodeReplicationConfigurationNotFound          = "ReplicationConfigurationNotFoundError"
	errCodeServerSideEncryptionConfigurationNotFound = "ServerSideEncryptionConfigurationNotFoundError"
	errCodeUnsupportedArgument                       = "UnsupportedArgument"
	// errCodeXNotImplemented, errCodeUnsupportedOperation are returned from third-party S3 API implementations.
	// References:
	//   https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/issues/14645.
	//   https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/pull/37801.
	errCodeXNotImplemented      = "XNotImplemented"
	errCodeUnsupportedOperation = "UnsupportedOperation"
)
