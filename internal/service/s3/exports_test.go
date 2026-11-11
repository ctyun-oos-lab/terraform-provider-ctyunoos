// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

// Exports for use in tests only.
var (
	ValidateObjectsNextMarker                       = dataSourceObjectsNextMarker
	FlattenSoftDeletedObjectContents                = flattenSoftDeletedObjectContents
	ResourceBucketACL                               = resourceBucketACL
	ResourceBucketSoftDelete                        = resourceBucketSoftDelete
	FindBucketSoftDelete                            = oosGetBucketSoftDelete
	ResourceBucketLifecycleConfiguration            = newBucketLifecycleConfigurationResource
	ResourceBucketLogging                           = resourceBucketLogging
	ResourceBucketObjectLockConfiguration           = resourceBucketObjectLockConfiguration
	ResourceBucketPolicy                            = resourceBucketPolicy
	ResourceBucketServerSideEncryptionConfiguration = resourceBucketServerSideEncryptionConfiguration
	ResourceObjectCopy                              = resourceObjectCopy
	DeleteAllObjectVersions                         = deleteAllObjectVersions
	EmptyBucket                                     = emptyBucket
	FindBucketACL                                   = findOOSBucketACL
	FindBucketLifecycleConfiguration                = findBucketLifecycleConfiguration
	FindBucketPolicy                                = findBucketPolicy
	FindLoggingEnabled                              = findLoggingEnabled
	FindObjectByBucketAndKey                        = findObjectByBucketAndKey
	FindObjectLockConfiguration                     = findObjectLockConfiguration
	FlattenOOSObjectLockConfigDataSourceRule        = flattenOOSObjectLockConfigDataSourceRule
	IsContentTypeAllowed                            = isContentTypeAllowed
	OOSDeleteBucketObjectLock                       = oosDeleteBucketObjectLock
	WithContentMD5                                  = withContentMD5
	FindServerSideEncryptionConfiguration           = findServerSideEncryptionConfiguration

	BucketPropagationTimeout       = bucketPropagationTimeout
	ErrCodeBucketAlreadyExists     = errCodeBucketAlreadyExists
	ErrCodeBucketAlreadyOwnedByYou = errCodeBucketAlreadyOwnedByYou
	ErrCodeNoSuchBucket            = errCodeNoSuchBucket
	ErrCodeNoSuchCORSConfiguration = errCodeNoSuchCORSConfiguration
	LifecycleRuleStatusDisabled    = lifecycleRuleStatusDisabled
	LifecycleRuleStatusEnabled     = lifecycleRuleStatusEnabled

	NewObjectARN   = newObjectARN
	ParseObjectARN = parseObjectARN

	ParseBucketACLResourceID = parseBucketACLResourceID
)

type (
	ObjectARN               = objectARN
	OOSLockRule             = oosLockRule
	OOSLockDefaultRetention = oosLockDefaultRetention
)
