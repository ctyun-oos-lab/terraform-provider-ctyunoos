// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

// Exports for use in tests only.
var (
	DataSourceAccessKeys                 = dataSourceAccessKeys
	DataSourceAccountLoginSecurityPolicy = dataSourceAccountLoginSecurityPolicy
	DataSourceAccountPasswordPolicy      = dataSourceAccountPasswordPolicy
	DataSourceAccountSummary             = dataSourceAccountSummary
	DataSourceGroup                      = dataSourceGroup
	DataSourceGroups                     = dataSourceGroups
	DataSourceMFADevices                 = dataSourceMFADevices
	DataSourcePolicies                   = dataSourcePolicies
	DataSourcePolicy                     = dataSourcePolicy
	DataSourcePolicyEntities             = dataSourcePolicyEntities
	DataSourceRole                       = dataSourceRole
	DataSourceRoles                      = dataSourceRoles
	DataSourceSAMLProvider               = dataSourceSAMLProvider
	DataSourceSAMLProviders              = dataSourceSAMLProviders
	DataSourceUser                       = dataSourceUser
	DataSourceUsers                      = dataSourceUsers
	NewUserGroupMembershipDataSource     = newUserGroupMembershipDataSource
	NewUserPolicyAttachmentsDataSource   = newUserPolicyAttachmentsDataSource
	NewGroupPolicyAttachmentsDataSource  = newGroupPolicyAttachmentsDataSource
	NewRolePolicyAttachmentsDataSource   = newRolePolicyAttachmentsDataSource
	NewVirtualMFADevicesDataSource       = newVirtualMFADevicesDataSource
	ResourceAccessKey                    = resourceAccessKey
	ResourceAccountLoginSecurityPolicy   = resourceAccountLoginSecurityPolicy
	ResourceAccountPasswordPolicy        = resourceAccountPasswordPolicy
	ResourceGroup                        = resourceGroup
	ResourceGroupPolicyAttachment        = resourceGroupPolicyAttachment
	ResourcePolicy                       = resourcePolicy
	ResourceRolePolicyAttachment         = resourceRolePolicyAttachment
	ResourceSAMLProvider                 = resourceSAMLProvider
	ResourceUser                         = resourceUser
	ResourceUserGroupMembership          = resourceUserGroupMembership
	ResourceUserLoginProfile             = resourceUserLoginProfile
	ResourceUserPolicyAttachment         = resourceUserPolicyAttachment
	ResourceVirtualMFADevice             = resourceVirtualMFADevice

	AccessKeyLastUsedReadResult                    = resourceAccessKeyLastUsedReadResult
	AccessKeyReadResult                            = resourceAccessKeyReadResult
	AccessKeysMutexKey                             = accessKeysMutexKey
	AccountPasswordPolicyUpdateValues              = accountPasswordPolicyUpdateValues
	AccountSummaryAttributeKeys                    = accountSummaryAttributeKeys
	AccountSummaryValue                            = accountSummaryValue
	FindAccessKeyByTwoPartKey                      = findAccessKeyByTwoPartKey
	FindAccessKeysByUser                           = findAccessKeysByUser
	FlattenAccessKeys                              = flattenAccessKeys
	FlattenAccessKey                               = flattenAccessKey
	FindAccountLoginSecurityPolicy                 = findAccountLoginSecurityPolicy
	FindAccountPasswordPolicy                      = findAccountPasswordPolicy
	AccountLoginSecurityPolicyUpdateValues         = accountLoginSecurityPolicyUpdateValues
	FindAttachedGroupPolicies                      = findAttachedGroupPolicies
	FindGroupPolicyAttachments                     = findGroupPolicyAttachments
	FindAttachedGroupPolicyByTwoPartKey            = findAttachedGroupPolicyByTwoPartKey
	FindAttachedRolePolicies                       = findAttachedRolePolicies
	FindRolePolicyAttachments                      = findRolePolicyAttachments
	FindAttachedRolePolicyByTwoPartKey             = findAttachedRolePolicyByTwoPartKey
	FindAttachedUserPolicies                       = findAttachedUserPolicies
	FindUserPolicyAttachments                      = findUserPolicyAttachments
	FindAttachedUserPolicyByTwoPartKey             = findAttachedUserPolicyByTwoPartKey
	FindGroupByName                                = findGroupByName
	FindGroupForUserByTwoPartKey                   = findGroupForUserByTwoPartKey
	FindPolicyByARN                                = findPolicyByARN
	FindPolicyWithDocumentByARN                    = findPolicyWithDocumentByARN
	FindPolicyDetailsByARN                         = findPolicyDetailsByARN
	FindPolicies                                   = findPolicies
	FindEntitiesForPolicy                          = findEntitiesForPolicy
	FindRoles                                      = findRoles
	FindGroups                                     = findGroups
	FindUsers                                      = findUsers
	FindGroupsForUser                              = findGroupsForUser
	FindMFADevicesForDataSource                    = findMFADevicesForDataSource
	FindVirtualMFADevicesForDataSource             = findVirtualMFADevicesForDataSource
	WithOOSIAMListPoliciesResponse                 = withOOSIAMListPoliciesResponse
	WithOOSIAMGetGroupResponse                     = withOOSIAMGetGroupResponse
	WithOOSIAMGetUserResponse                      = withOOSIAMGetUserResponse
	OOSIAMUserIPLastUsedMetadataKey                = oosIAMUserIPLastUsedMetadataKey
	FindSAMLProviderByARN                          = findSAMLProviderByARN
	FindSAMLProviders                              = findSAMLProviders
	ListSAMLProvidersPage                          = listSAMLProvidersPage
	NameFromSAMLProviderARN                        = nameFromSAMLProviderARN
	FindUserByName                                 = findUserByName
	KeyValueTags                                   = keyValueTags
	FindVirtualMFADeviceBySerialNumber             = findVirtualMFADeviceBySerialNumber
	FilterGroupsByNameRegex                        = filterGroupsByNameRegex
	FilterPoliciesByNameRegex                      = filterPoliciesByNameRegex
	FilterRolesByNameRegex                         = filterRolesByNameRegex
	FilterSAMLProvidersByNameRegex                 = filterSAMLProvidersByNameRegex
	FilterUsersByNameRegex                         = filterUsersByNameRegex
	FlattenAccountLoginSecurityPolicyForDataSource = flattenAccountLoginSecurityPolicyForDataSource
	FlattenAccountPasswordPolicyForDataSource      = flattenAccountPasswordPolicyForDataSource
	FlattenGroupAttachedPolicies                   = flattenGroupAttachedPolicies
	FlattenUserAttachedPolicies                    = flattenUserAttachedPolicies
	FlattenRoleAttachedPolicies                    = flattenRoleAttachedPolicies
	FlattenGroups                                  = flattenGroups
	FlattenGroupsForUser                           = flattenGroupsForUser
	FlattenMFADeviceForDataSource                  = flattenMFADeviceForDataSource
	FlattenMFADevicesForDataSource                 = flattenMFADevicesForDataSource
	FlattenPolicies                                = flattenPolicies
	FlattenPolicy                                  = flattenPolicy
	FlattenPolicyGroups                            = flattenPolicyGroups
	FlattenPolicyRoles                             = flattenPolicyRoles
	FlattenPolicyUsers                             = flattenPolicyUsers
	FindPolicyARNByNameFromList                    = findPolicyARNByNameFromList
	ResourcePolicyFlattenPolicyDocument            = resourcePolicyFlattenPolicyDocument
	FlattenRoles                                   = flattenRoles
	FlattenSAMLProviderListEntry                   = flattenSAMLProviderListEntry
	FlattenUsers                                   = flattenUsers
	FlattenVirtualMFADevices                       = flattenVirtualMFADevices

	AttachPolicyToUser                      = attachPolicyToUser
	CheckPwdPolicy                          = checkPwdPolicy
	CheckPwdPolicyWithPolicy                = checkPwdPolicyWithPolicy
	GeneratePassword                        = generatePassword
	GeneratePasswordWithPolicy              = generatePasswordWithPolicy
	UserLoginProfileCreateValues            = userLoginProfileCreateValues
	IsValidPolicyAWSPrincipal               = isValidPolicyAWSPrincipal // nosemgrep:ci.aws-in-var-name
	ListGroupsForUserPages                  = listGroupsForUserPages
	LockAccessKeys                          = lockAccessKeys
	IAMClientWithStaticCredentials          = iamClientWithStaticCredentials
	OOSIAMQuery                             = oosIAMQuery
	TestProbeGetUserWithStaticCredentials   = testProbeGetUserWithStaticCredentials
	TestProbeListUsersWithStaticCredentials = testProbeListUsersWithStaticCredentials
)

type (
	AccountLoginSecurityPolicyInput       = accountLoginSecurityPolicyInput
	GetAccountLoginSecurityPolicyResponse = getAccountLoginSecurityPolicyResponse
	IAMPolicyStatementConditionSet        = iamPolicyStatementConditionSet
	OOSAttachedPolicyMetadata             = oosAttachedPolicyMetadata
	OOSListGroupsMetadata                 = oosListGroupsMetadata
)
