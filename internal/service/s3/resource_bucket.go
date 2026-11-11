// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/create"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/flex"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2/importer"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/verify"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	sdkid "github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const (
	// General timeout for S3 bucket changes to propagate.
	// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html#ConsistencyModel.
	bucketPropagationTimeout = 2 * time.Minute
)

// @SDKResource("ctyun_oos_bucket", name="Bucket")
// @IdentityAttribute("bucket")
// @CustomImport
// @Testing(idAttrDuplicates="bucket", hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceBucket() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketCreate,
		ReadWithoutTimeout:   resourceBucketRead,
		UpdateWithoutTimeout: resourceBucketUpdate,
		DeleteWithoutTimeout: resourceBucketDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.Import(ctx, rd, meta); err != nil {
					return nil, err
				}

				rd.Set(names.AttrForceDestroy, false)

				return []*schema.ResourceData{rd}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(20 * time.Minute),
			Read:   schema.DefaultTimeout(20 * time.Minute),
			Update: schema.DefaultTimeout(20 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"acl": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"private", "public-read", "public-read-write"}, false),
				Deprecated:   "acl is deprecated. Use the ctyun_oos_bucket_acl resource instead.",
			},
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrBucket: {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			names.AttrBucketPrefix: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrBucket},
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63-sdkid.UniqueIDSuffixLength),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			names.AttrForceDestroy: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"metadata_location_constraint": {
				// The index position is returned by OOS GetBucketLocation, so allow the
				// service value to be populated into state.
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrLocation: {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"data_location_constraint": {
				// The data position; per OOS PutBucket this CAN be modified, so it is
				// not ForceNew. OOS always assigns a data location to the bucket (default Local/Allowed),
				// so Read sees it even when the user does not declare it. It is therefore marked Computed,
				// making "computed by the server when undeclared" a legitimate state and avoiding a
				// permanent diff after apply.
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrType: {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      oosDefaultRegionType,
							ValidateFunc: validation.StringInSlice([]string{"Local", "Specified"}, false),
						},
						"locations": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"schedule_strategy": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      oosDefaultScheduleStrategy,
							ValidateFunc: validation.StringInSlice([]string{"Allowed", "NotAllowed"}, false),
						},
					},
				},
			},
			"lifecycle_rule": {
				Type:       schema.TypeList,
				Optional:   true,
				Computed:   true,
				Deprecated: "lifecycle_rule is deprecated. Use the ctyun_oos_bucket_lifecycle_configuration resource instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrEnabled: {
							Type:     schema.TypeBool,
							Required: true,
						},
						"expiration": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"date": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validBucketLifecycleTimestamp,
									},
									"days": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntAtLeast(1),
									},
								},
							},
						},
						names.AttrID: {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.StringLenBetween(1, 255),
						},
						names.AttrPrefix: {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringLenBetween(0, 1024),
						},
						// OOS allows at most one transition per rule.
						"transition": {
							Type:     schema.TypeSet,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"date": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validBucketLifecycleTimestamp,
									},
									"days": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntAtLeast(1),
									},
									names.AttrStorageClass: {
										Type:         schema.TypeString,
										Required:     true,
										ValidateFunc: validation.StringInSlice([]string{string(types.TransitionStorageClassStandardIa)}, false),
									},
								},
							},
						},
					},
				},
			},
			"logging": {
				Type:       schema.TypeList,
				Optional:   true,
				Computed:   true,
				MaxItems:   1,
				Deprecated: "logging is deprecated. Use the ctyun_oos_bucket_logging resource instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"target_bucket": {
							Type:     schema.TypeString,
							Required: true,
						},
						"target_prefix": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"object_lock_configuration": {
				Type:       schema.TypeList,
				Optional:   true,
				Computed:   true,
				MaxItems:   1,
				Deprecated: "object_lock_configuration is deprecated. Use the top-level parameter object_lock_enabled and the ctyun_oos_bucket_object_lock_configuration resource instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"object_lock_enabled": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{string(types.ObjectLockEnabledEnabled), "Disabled"}, false),
							Deprecated:   "object_lock_enabled is deprecated. Use the ctyun_oos_bucket_object_lock_configuration resource instead.",
						},
						names.AttrRule: {
							Type:       schema.TypeList,
							Optional:   true,
							Deprecated: "rule is deprecated. Use the ctyun_oos_bucket_object_lock_configuration resource instead.",
							MaxItems:   1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"default_retention": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"days": {
													Type:          schema.TypeInt,
													Optional:      true,
													ValidateFunc:  validation.IntBetween(1, 36500),
													ConflictsWith: []string{"object_lock_configuration.0.rule.0.default_retention.0.years"},
												},
												names.AttrMode: {
													Type:         schema.TypeString,
													Required:     true,
													ValidateFunc: validation.StringInSlice([]string{string(types.ObjectLockRetentionModeCompliance)}, false),
												},
												"years": {
													Type:          schema.TypeInt,
													Optional:      true,
													ValidateFunc:  validation.IntBetween(1, 100),
													ConflictsWith: []string{"object_lock_configuration.0.rule.0.default_retention.0.days"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			names.AttrPolicy: {
				Type:                  schema.TypeString,
				Optional:              true,
				Computed:              true,
				Deprecated:            "policy is deprecated. Use the ctyun_oos_bucket_policy resource instead.",
				ValidateFunc:          validation.StringIsJSON,
				DiffSuppressFunc:      verify.SuppressEquivalentPolicyDiffs,
				DiffSuppressOnRefresh: true,
				StateFunc: func(v any) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
			"server_side_encryption_configuration": {
				Type:       schema.TypeList,
				MaxItems:   1,
				Optional:   true,
				Computed:   true,
				Deprecated: "server_side_encryption_configuration is deprecated. Use the ctyun_oos_bucket_server_side_encryption_configuration resource instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrRule: {
							Type:     schema.TypeList,
							MaxItems: 1,
							Required: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"apply_server_side_encryption_by_default": {
										Type:     schema.TypeList,
										MaxItems: 1,
										Required: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												// OOS only supports AES256 (no KMS key).
												"sse_algorithm": {
													Type:         schema.TypeString,
													Required:     true,
													ValidateFunc: validation.StringInSlice([]string{string(types.ServerSideEncryptionAes256)}, false),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceBucketCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	c := meta.(*conns.AWSClient)
	conn := c.S3Client(ctx)

	// bucketName
	bucket := create.NewNameGenerator(
		create.WithConfiguredName(d.Get(names.AttrBucket).(string)),
		create.WithConfiguredPrefix(d.Get(names.AttrBucketPrefix).(string)),
	).Generate(ctx)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	in := oosCreateBucketInput{
		bucket: bucket,
		acl:    d.Get("acl").(string),
	}

	if in.acl == "" {
		in.acl = oosDefaultACL
	}

	if v, ok := d.GetOk("metadata_location_constraint"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
		tfMap := v.([]any)[0].(map[string]any)
		in.metadataLocation = aws.String(tfMap[names.AttrLocation].(string))
	}

	if v, ok := d.GetOk("data_location_constraint"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
		tfMap := v.([]any)[0].(map[string]any)
		in.hasDataConstraint = true
		in.dataType = tfMap[names.AttrType].(string)
		in.dataScheduleStreg = tfMap["schedule_strategy"].(string)
		in.dataLocations = flex.ExpandStringValueList(tfMap["locations"].([]any))
	}

	err := oosPutBucket(ctx, conn, in)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s): %s", bucket, err)
	}

	d.SetId(bucket)

	_, err = tfresource.RetryWhenNotFound(ctx, d.Timeout(schema.TimeoutCreate), func(ctx context.Context) (any, error) {
		return FindOOSBucket(ctx, conn, d.Id())
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceBucketUpdateCore(ctx, d, meta)...)
}

func resourceBucketRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	c := meta.(*conns.AWSClient)
	conn := c.S3Client(ctx)

	// HeadBucket is standard S3 and OOS-compatible; it confirms existence.
	_, err := FindOOSBucket(ctx, conn, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrBucket, d.Id())
	d.Set(names.AttrBucketPrefix, create.NamePrefixFromName(d.Id()))
	d.Set(names.AttrARN, oosBucketARN(d.Id()))

	//
	// Bucket Location (OOS-specific GET /?location): read back the metadata/data
	// location constraints into state for drift detection.
	//
	cfg, err := oosGetBucketLocation(ctx, conn, d.Id())
	if err != nil {
		if tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusMethodNotAllowed) && tfawserr.ErrCodeEquals(err, errCodeMethodNotAllowed) {
			cfg = nil
		} else {
			return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) location: %s", d.Id(), err)
		}
	}

	if err := d.Set("metadata_location_constraint", flattenMetadataLocationConstraint(cfg)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting metadata_location_constraint: %s", err)
	}
	if err := d.Set("data_location_constraint", flattenDataLocationConstraint(cfg)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting data_location_constraint: %s", err)
	}

	//
	// Bucket ACL (standard S3 GetBucketAcl): map the grants back to a canned ACL.
	//
	acl, err := findOOSBucketACL(ctx, conn, d.Id())
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) ACL: %s", d.Id(), err)
	}
	d.Set("acl", cannedACLFromGrants(acl))

	//
	// Bucket Policy.
	//
	policy, err := retryWhenNoSuchBucketError(ctx, d.Timeout(schema.TimeoutRead), func(ctx context.Context) (string, error) {
		return findBucketPolicy(ctx, conn, d.Id())
	})

	// The call to HeadBucket above can occasionally return no error (i.e. NoSuchBucket)
	// after a bucket has been deleted (eventual consistency woes :/), thus, when making extra S3 API calls
	// such as GetBucketPolicy, the error should be caught for non-new buckets as follows.
	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	switch {
	case err == nil:
		policyToSet, err := verify.PolicyToSet(d.Get(names.AttrPolicy).(string), policy)
		if err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}

		d.Set(names.AttrPolicy, policyToSet)
	case retry.NotFound(err), tfawserr.ErrCodeEquals(err, errCodeMethodNotAllowed, errCodeNotImplemented, errCodeXNotImplemented):
		d.Set(names.AttrPolicy, nil)
	default:
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) policy: %s", d.Id(), err)
	}

	//
	// Bucket Logging.
	//
	loggingEnabled, err := retryWhenNoSuchBucketError(ctx, d.Timeout(schema.TimeoutRead), func(ctx context.Context) (*oosLoggingEnabled, error) {
		return oosGetBucketLogging(ctx, conn, d.Id())
	})

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	switch {
	case err == nil:
		if err := d.Set("logging", flattenBucketLoggingEnabled(loggingEnabled)); err != nil {
			return sdkdiag.AppendErrorf(diags, "setting logging: %s", err)
		}
	case retry.NotFound(err), tfawserr.ErrCodeEquals(err, errCodeMethodNotAllowed, errCodeNotImplemented, errCodeXNotImplemented):
		d.Set("logging", nil)
	default:
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) logging: %s", d.Id(), err)
	}

	//
	// Bucket Lifecycle Configuration.
	//
	lifecycleRules, err := retryWhenNoSuchBucketError(ctx, d.Timeout(schema.TimeoutRead), func(ctx context.Context) ([]oosLifecycleRule, error) {
		return oosGetBucketLifecycle(ctx, conn, d.Id())
	})

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	switch {
	case err == nil:
		if err := d.Set("lifecycle_rule", flattenBucketLifecycleRules(lifecycleRules)); err != nil {
			return sdkdiag.AppendErrorf(diags, "setting lifecycle_rule: %s", err)
		}
	case retry.NotFound(err), tfawserr.ErrCodeEquals(err, errCodeNoSuchLifecycleConfiguration, errCodeMethodNotAllowed, errCodeNotImplemented, errCodeXNotImplemented):
		d.Set("lifecycle_rule", nil)
	default:
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) lifecycle configuration: %s", d.Id(), err)
	}

	//
	// Bucket Server-side Encryption Configuration.
	//
	encryptionConfiguration, err := retryWhenNoSuchBucketError(ctx, d.Timeout(schema.TimeoutRead), func(ctx context.Context) (*oosServerSideEncryptionConfiguration, error) {
		return oosGetBucketEncryption(ctx, conn, d.Id())
	})

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	switch {
	case err == nil:
		if err := d.Set("server_side_encryption_configuration", flattenBucketServerSideEncryptionConfiguration(encryptionConfiguration)); err != nil {
			return sdkdiag.AppendErrorf(diags, "setting server_side_encryption_configuration: %s", err)
		}
	case retry.NotFound(err), tfawserr.ErrCodeContains(err, errCodeServerSideEncryptionConfigurationNotFound), tfawserr.ErrCodeEquals(err, errCodeNoSuchServerSideEncryptionRule, errCodeMethodNotAllowed, errCodeNotImplemented, errCodeXNotImplemented, errCodeUnsupportedOperation):
		d.Set("server_side_encryption_configuration", nil)
	default:
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) server-side encryption configuration: %s", d.Id(), err)
	}

	//
	// Bucket Object Lock Configuration.
	//
	objLockConfig, err := retryWhenNoSuchBucketError(ctx, d.Timeout(schema.TimeoutRead), func(ctx context.Context) (*oosLockConfiguration, error) {
		return oosGetBucketObjectLock(ctx, conn, d.Id())
	})

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		log.Printf("[WARN] OOS Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	switch {
	case err == nil:
		if err := d.Set("object_lock_configuration", flattenObjectLockConfiguration(objLockConfig)); err != nil {
			return sdkdiag.AppendErrorf(diags, "setting object_lock_configuration: %s", err)
		}
	case retry.NotFound(err), tfawserr.ErrCodeContains(err, errCodeObjectLockConfigurationNotFound), tfawserr.ErrCodeEquals(err, errCodeMethodNotAllowed, errCodeNotImplemented, errCodeXNotImplemented):
		d.Set("object_lock_configuration", nil)
	default:
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) object lock configuration: %s", d.Id(), err)
	}

	return diags
}

func resourceBucketUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	unlock := lockBucketControlPlane(d.Id())
	defer unlock()

	return resourceBucketUpdateCore(ctx, d, meta)
}

func resourceBucketUpdateCore(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	// Note: Order of argument updates below is important.

	// Per the OOS PutBucket API, the metadata position
	// (metadata_location_constraint), data position (data_location_constraint),
	// and ACL of an existing bucket can be modified by re-issuing PUT Bucket.
	//
	// Skipped on create: resourceBucketCreate already issues the initial PutBucket
	// with these arguments, so re-sending them here would be redundant.
	if !d.IsNewResource() && (d.HasChange("metadata_location_constraint") || d.HasChange("data_location_constraint") || d.HasChange("acl")) {
		in := oosCreateBucketInput{
			bucket: d.Id(),
			acl:    d.Get("acl").(string),
		}

		if v, ok := d.GetOk("metadata_location_constraint"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			tfMap := v.([]any)[0].(map[string]any)
			in.metadataLocation = aws.String(tfMap[names.AttrLocation].(string))
		}

		if v, ok := d.GetOk("data_location_constraint"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			tfMap := v.([]any)[0].(map[string]any)
			in.hasDataConstraint = true
			in.dataType = tfMap[names.AttrType].(string)
			in.dataScheduleStreg = tfMap["schedule_strategy"].(string)
			in.dataLocations = flex.ExpandStringValueList(tfMap["locations"].([]any))
		}

		if err := oosPutBucket(ctx, conn, in); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating OOS Bucket (%s): %s", d.Id(), err)
		}
	}

	//
	// Bucket Policy.
	//
	if d.HasChange(names.AttrPolicy) {
		policy, err := structure.NormalizeJsonString(d.Get(names.AttrPolicy).(string))
		if err != nil {
			return sdkdiag.AppendFromErr(diags, err)
		}

		if policy == "" {
			_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return conn.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
					Bucket: aws.String(d.Id()),
				})
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket (%s) policy: %s", d.Id(), err)
			}
		} else {
			_, err = tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return nil, oosPutBucketPolicy(ctx, conn, d.Id(), policy)
			}, errCodeMalformedPolicy, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) policy: %s", d.Id(), err)
			}
		}
	}

	//
	// Bucket Logging.
	//
	if d.HasChange("logging") {
		var enabled *oosLoggingEnabled

		if v, ok := d.GetOk("logging"); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
			tfMap := v.([]any)[0].(map[string]any)
			enabled = &oosLoggingEnabled{}

			if v, ok := tfMap["target_bucket"].(string); ok {
				enabled.TargetBucket = v
			}

			if v, ok := tfMap["target_prefix"].(string); ok {
				enabled.TargetPrefix = v
			}
		}

		_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
			return nil, oosPutBucketLogging(ctx, conn, d.Id(), enabled)
		}, errCodeNoSuchBucket)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) logging: %s", d.Id(), err)
		}
	}

	//
	// Bucket Lifecycle Configuration.
	//
	if d.HasChange("lifecycle_rule") {
		if v, ok := d.GetOk("lifecycle_rule"); !ok || len(v.([]any)) == 0 {
			_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return conn.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
					Bucket: aws.String(d.Id()),
				})
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket (%s) lifecycle configuration: %s", d.Id(), err)
			}
		} else {
			rules, err := expandBucketLifecycleRules(v.([]any))
			if err != nil {
				return sdkdiag.AppendFromErr(diags, err)
			}

			_, err = tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return nil, oosPutBucketLifecycle(ctx, conn, d.Id(), rules)
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) lifecycle configuration: %s", d.Id(), err)
			}
		}
	}

	//
	// Bucket Server-side Encryption Configuration.
	//
	if d.HasChange("server_side_encryption_configuration") {
		if v, ok := d.GetOk("server_side_encryption_configuration"); ok && len(v.([]any)) > 0 {
			algorithm := expandBucketServerSideEncryptionAlgorithm(v.([]any))

			_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return nil, oosPutBucketEncryption(ctx, conn, d.Id(), algorithm)
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) server-side encryption configuration: %s", d.Id(), err)
			}
		}
		// OOS does not allow removing bucket server-side encryption once configured.
		// Removing the deprecated inline block is therefore a remote no-op; the
		// subsequent Read will repopulate state from the server if encryption exists.
	}

	//
	// Bucket Object Lock Configuration.
	//
	if d.HasChange("object_lock_configuration") {
		if v, ok := d.GetOk("object_lock_configuration"); !ok || len(v.([]any)) == 0 || v.([]any)[0] == nil {
			// OOS allows deleting an unenabled object lock configuration.
			_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return nil, oosDeleteBucketObjectLock(ctx, conn, d.Id())
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket (%s) object lock configuration: %s", d.Id(), err)
			}
		} else {
			cfg := expandBucketObjectLockConfiguration(v.([]any))

			_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, d.Timeout(schema.TimeoutUpdate), func(ctx context.Context) (any, error) {
				return nil, oosPutBucketObjectLock(ctx, conn, d.Id(), *cfg)
			}, errCodeNoSuchBucket)

			if err != nil {
				return sdkdiag.AppendErrorf(diags, "putting OOS Bucket (%s) object lock configuration: %s", d.Id(), err)
			}
		}
	}

	return append(diags, resourceBucketRead(ctx, d, meta)...)
}

func resourceBucketDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	unlock := lockBucketControlPlane(d.Id())
	defer unlock()

	return resourceBucketDeleteCore(ctx, d, meta)
}

func resourceBucketDeleteCore(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	log.Printf("[INFO] Deleting OOS Bucket: %s", d.Id())
	_, err := conn.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(d.Id()),
	})

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		return diags
	}

	if tfawserr.ErrCodeEquals(err, errCodeBucketNotEmpty) {
		if d.Get(names.AttrForceDestroy).(bool) {
			nObjects, emptyErr := emptyBucket(ctx, conn, d.Id(), true)
			if emptyErr != nil {
				return sdkdiag.AppendErrorf(diags, "emptying OOS Bucket (%s): %s", d.Id(), emptyErr)
			}

			log.Printf("[INFO] Deleted %d object(s) from OOS Bucket (%s)", nObjects, d.Id())

			_, err = conn.DeleteBucket(ctx, &s3.DeleteBucketInput{
				Bucket: aws.String(d.Id()),
			})
			if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
				return diags
			}
		}
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket (%s): %s", d.Id(), err)
	}

	_, err = tfresource.RetryUntilNotFound(ctx, d.Timeout(schema.TimeoutDelete), func(ctx context.Context) (any, error) {
		return FindOOSBucket(ctx, conn, d.Id())
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket (%s) delete: %s", d.Id(), err)
	}

	return diags
}

//
// Bucket Logging.
//

func flattenBucketLoggingEnabled(apiObject *oosLoggingEnabled) []any {
	if apiObject == nil {
		return []any{}
	}

	m := map[string]any{
		"target_bucket": apiObject.TargetBucket,
		"target_prefix": apiObject.TargetPrefix,
	}

	return []any{m}
}

//
// Bucket Lifecycle Configuration.
//

func expandBucketLifecycleRules(tfList []any) ([]oosLifecycleRule, error) {
	if len(tfList) == 0 || tfList[0] == nil {
		return nil, nil
	}

	rules := make([]oosLifecycleRule, 0, len(tfList))

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]any)
		if !ok {
			continue
		}

		rule := oosLifecycleRule{
			Prefix: tfMap[names.AttrPrefix].(string),
		}

		if v, ok := tfMap[names.AttrID].(string); ok && v != "" {
			rule.ID = v
		} else {
			rule.ID = sdkid.PrefixedUniqueId("tf-s3-lifecycle-")
		}

		if v, ok := tfMap[names.AttrEnabled].(bool); ok && v {
			rule.Status = string(types.ExpirationStatusEnabled)
		} else {
			rule.Status = string(types.ExpirationStatusDisabled)
		}

		if v, ok := tfMap["expiration"].([]any); ok && len(v) > 0 && v[0] != nil {
			expiration, err := expandBucketLifecycleExpiration(v[0].(map[string]any))
			if err != nil {
				return nil, fmt.Errorf("lifecycle rule %q expiration: %w", rule.ID, err)
			}
			rule.Expiration = expiration
		}

		if v, ok := tfMap["transition"].([]any); ok && len(v) > 0 && v[0] != nil {
			transition, err := expandBucketLifecycleTransition(v[0].(map[string]any))
			if err != nil {
				return nil, fmt.Errorf("lifecycle rule %q transition: %w", rule.ID, err)
			}
			rule.Transition = transition
		}

		// OOS requires each rule to carry at least one of Expiration/Transition.
		if rule.Expiration == nil && rule.Transition == nil {
			return nil, fmt.Errorf("lifecycle rule %q: at least one of expiration or transition is required", rule.ID)
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

func expandBucketLifecycleExpiration(tfMap map[string]any) (*oosLifecycleExpiry, error) {
	apiObject := &oosLifecycleExpiry{}

	date, _ := tfMap["date"].(string)
	days, hasDays := tfMap["days"].(int)

	switch {
	case date != "" && hasDays && days > 0:
		return nil, fmt.Errorf("only one of date or days can be specified")
	case date != "":
		apiObject.Date = normalizeLifecycleRFC3339(date)
	case hasDays && days > 0:
		apiObject.Days = aws.Int32(int32(days))
	default:
		return nil, fmt.Errorf("one of date or days must be specified")
	}

	return apiObject, nil
}

func expandBucketLifecycleTransition(tfMap map[string]any) (*oosLifecycleTransit, error) {
	apiObject := &oosLifecycleTransit{}

	date, _ := tfMap["date"].(string)
	days, hasDays := tfMap["days"].(int)

	switch {
	case date != "" && hasDays && days > 0:
		return nil, fmt.Errorf("only one of date or days can be specified")
	case date != "":
		apiObject.Date = normalizeLifecycleRFC3339(date)
	case hasDays && days > 0:
		apiObject.Days = aws.Int32(int32(days))
	default:
		return nil, fmt.Errorf("one of date or days must be specified")
	}

	if v, ok := tfMap[names.AttrStorageClass].(string); ok && v != "" {
		apiObject.StorageClass = v
	}

	return apiObject, nil
}

func flattenBucketLifecycleRules(rules []oosLifecycleRule) []any {
	if len(rules) == 0 {
		return []any{}
	}

	tfList := make([]any, 0, len(rules))

	for _, rule := range rules {
		tfMap := map[string]any{
			names.AttrID:      rule.ID,
			names.AttrPrefix:  rule.Prefix,
			names.AttrEnabled: rule.Status == string(types.ExpirationStatusEnabled),
		}

		if v := rule.Expiration; v != nil {
			tfMap["expiration"] = flattenBucketLifecycleExpiration(v)
		}

		if v := rule.Transition; v != nil {
			tfMap["transition"] = flattenBucketLifecycleTransition(v)
		}

		tfList = append(tfList, tfMap)
	}

	return tfList
}

func flattenBucketLifecycleExpiration(apiObject *oosLifecycleExpiry) []any {
	if apiObject == nil {
		return []any{}
	}

	tfMap := make(map[string]any)

	if apiObject.Date != "" {
		tfMap["date"] = lifecycleDateToSchema(apiObject.Date)
	}

	if v := apiObject.Days; v != nil {
		tfMap["days"] = aws.ToInt32(v)
	}

	return []any{tfMap}
}

func flattenBucketLifecycleTransition(apiObject *oosLifecycleTransit) []any {
	if apiObject == nil {
		return []any{}
	}

	tfMap := map[string]any{
		names.AttrStorageClass: apiObject.StorageClass,
	}

	if apiObject.Date != "" {
		tfMap["date"] = lifecycleDateToSchema(apiObject.Date)
	}

	if v := apiObject.Days; v != nil {
		tfMap["days"] = aws.ToInt32(v)
	}

	return []any{tfMap}
}

// lifecycleDateToSchema normalizes an OOS ISO8601 lifecycle date to the
// provider's canonical UTC midnight representation. If parsing fails it returns
// the value unchanged.
func lifecycleDateToSchema(v string) string {
	return normalizeLifecycleRFC3339(v)
}

//
// Bucket Server-side Encryption Configuration.
//

// expandBucketServerSideEncryptionAlgorithm pulls the single SSEAlgorithm out of
// the (OOS-trimmed) server_side_encryption_configuration block. OOS only supports
// a single rule carrying ApplyServerSideEncryptionByDefault.SSEAlgorithm, so we
// return just that value rather than the SDK's []ServerSideEncryptionRule.
func expandBucketServerSideEncryptionAlgorithm(l []any) string {
	if len(l) == 0 || l[0] == nil {
		return ""
	}

	tfMap, ok := l[0].(map[string]any)
	if !ok {
		return ""
	}

	rules, ok := tfMap[names.AttrRule].([]any)
	if !ok || len(rules) == 0 || rules[0] == nil {
		return ""
	}

	ruleMap, ok := rules[0].(map[string]any)
	if !ok {
		return ""
	}

	def, ok := ruleMap["apply_server_side_encryption_by_default"].([]any)
	if !ok || len(def) == 0 || def[0] == nil {
		return ""
	}

	defMap, ok := def[0].(map[string]any)
	if !ok {
		return ""
	}

	if v, ok := defMap["sse_algorithm"].(string); ok {
		return v
	}

	return ""
}

func flattenBucketServerSideEncryptionConfiguration(apiObject *oosServerSideEncryptionConfiguration) []any {
	if apiObject == nil || apiObject.Rule == nil || apiObject.Rule.ApplyServerSideEncryptionByDefault == nil {
		return []any{}
	}

	m := map[string]any{
		names.AttrRule: []any{
			map[string]any{
				"apply_server_side_encryption_by_default": []any{
					map[string]any{
						"sse_algorithm": apiObject.Rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm,
					},
				},
			},
		},
	}

	return []any{m}
}

//
// Bucket Object Lock Configuration.
//

func expandBucketObjectLockConfiguration(l []any) *oosLockConfiguration {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	tfMap, ok := l[0].(map[string]any)
	if !ok {
		return nil
	}

	apiObject := &oosLockConfiguration{}

	if v, ok := tfMap["object_lock_enabled"].(string); ok && v != "" {
		apiObject.ObjectLockEnabled = v
	}

	if v, ok := tfMap[names.AttrRule].([]any); ok && len(v) > 0 {
		tfMap := v[0].(map[string]any)

		if v, ok := tfMap["default_retention"].([]any); ok && len(v) > 0 && v[0] != nil {
			tfMap := v[0].(map[string]any)

			dr := &oosLockDefaultRetention{}

			if v, ok := tfMap[names.AttrMode].(string); ok && v != "" {
				dr.Mode = v
			}
			if v, ok := tfMap["days"].(int); ok && v > 0 {
				dr.Days = aws.Int32(int32(v))
			}
			if v, ok := tfMap["years"].(int); ok && v > 0 {
				dr.Years = aws.Int32(int32(v))
			}

			apiObject.Rule = &oosLockRule{DefaultRetention: dr}
		}
	}

	return apiObject
}

func flattenObjectLockConfiguration(apiObject *oosLockConfiguration) []any {
	if apiObject == nil {
		return []any{}
	}

	m := map[string]any{
		"object_lock_enabled": apiObject.ObjectLockEnabled,
	}

	if apiObject.Rule != nil && apiObject.Rule.DefaultRetention != nil {
		dr := apiObject.Rule.DefaultRetention
		tfMap := map[string]any{
			"default_retention": []any{
				map[string]any{
					names.AttrMode: dr.Mode,
					"days":         aws.ToInt32(dr.Days),
					"years":        aws.ToInt32(dr.Years),
				},
			},
		}

		m[names.AttrRule] = []any{tfMap}
	}

	return []any{m}
}

// oosBucketARN builds the ARN for an OOS bucket. OOS is not an AWS partition, so
// the standard aws-partition ARN (arn:aws:s3:::<bucket>) does not apply; OOS uses
// arn:ctyun:oos:::<bucket> (no region, no account ID).
func oosBucketARN(bucket string) string {
	return fmt.Sprintf("arn:ctyun:oos:::%s", bucket)
}

func validBucketLifecycleTimestamp(v any, k string) (ws []string, errors []error) {
	value := v.(string)
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		errors = append(errors, fmt.Errorf(
			"%q must be a valid ISO8601 timestamp", value))
		return
	}

	expected := t.UTC().Format("2006-01-02T00:00:00.000Z")
	if value != expected {
		errors = append(errors, fmt.Errorf(
			"%q must be a UTC midnight timestamp in the exact form %q", value, expected))
	}

	return
}

func FindOOSBucket(ctx context.Context, conn *s3.Client, bucket string, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	input := s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}

	output, err := conn.HeadBucket(ctx, &input, optFns...)

	if tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) || tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) || errs.Contains(err, errCodeNoSuchBucket) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}

func retryWhenNoSuchBucketError[T any](ctx context.Context, timeout time.Duration, f func(context.Context) (T, error)) (T, error) {
	return tfresource.RetryWhenAWSErrCodeEquals(ctx, timeout, f, errCodeNoSuchBucket)
}

// cannedACLFromGrants does a best-effort reverse mapping of an ACL grant list to
// a canned ACL string. OOS PutBucket only accepts canned ACLs (private,
// public-read, public-read-write), so GetBucketAcl grants are matched back to
// the closest canned value. Unknown patterns fall back to "private".
// cannedACLFromGrants maps an OOS GetBucketAcl response back to a canned ACL.
//
// OOS does not use the standard S3 multi-grant model. It encodes the canned
// level as a single Permission on the AllUsers group grant (see the GET Bucket
// ACL API):
//
//   - Permission READ          -> public-read
//   - Permission FULL_CONTROL  -> public-read-write
//   - empty Permission, or no AllUsers grant -> private
func cannedACLFromGrants(acl *s3.GetBucketAclOutput) string {
	if acl == nil {
		return "private"
	}

	const allUsersURI = "http://acs.amazonaws.com/groups/global/AllUsers"

	for _, g := range acl.Grants {
		if g.Grantee == nil || g.Grantee.URI == nil || aws.ToString(g.Grantee.URI) != allUsersURI {
			continue
		}
		switch g.Permission {
		case types.PermissionRead:
			return "public-read"
		case types.PermissionFullControl:
			return "public-read-write"
		}
	}

	return "private"
}

// findOOSBucketACL reads a bucket's ACL via the standard S3 GetBucketAcl API.
func findOOSBucketACL(ctx context.Context, conn *s3.Client, bucket string) (*s3.GetBucketAclOutput, error) {
	input := s3.GetBucketAclInput{
		Bucket: aws.String(bucket),
	}

	output, err := conn.GetBucketAcl(ctx, &input)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		return nil, &retry.NotFoundError{LastError: err}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}

// flattenMetadataLocationConstraint maps the OOS GetBucketLocation response into
// the metadata_location_constraint block shape.
func flattenMetadataLocationConstraint(cfg *oosBucketConfiguration) []any {
	if cfg == nil || cfg.MetadataLocationConstraint == nil {
		return []any{}
	}

	return []any{
		map[string]any{
			names.AttrLocation: cfg.MetadataLocationConstraint.Location,
		},
	}
}

// flattenDataLocationConstraint maps the OOS GetBucketLocation response into the
// data_location_constraint block shape.
func flattenDataLocationConstraint(cfg *oosBucketConfiguration) []any {
	if cfg == nil || cfg.DataLocationConstraint == nil {
		return []any{}
	}

	dlc := cfg.DataLocationConstraint
	return []any{
		map[string]any{
			names.AttrType:      dlc.Type,
			"locations":         dlc.Locations,
			"schedule_strategy": dlc.ScheduleStrategy,
		},
	}
}
