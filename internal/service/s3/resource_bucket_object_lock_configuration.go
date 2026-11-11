// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_oos_bucket_object_lock_configuration", name="Bucket Object Lock Configuration")
func resourceBucketObjectLockConfiguration() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketObjectLockConfigurationCreate,
		ReadWithoutTimeout:   resourceBucketObjectLockConfigurationRead,
		UpdateWithoutTimeout: resourceBucketObjectLockConfigurationUpdate,
		DeleteWithoutTimeout: resourceBucketObjectLockConfigurationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"object_lock_enabled": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{string(types.ObjectLockEnabledEnabled), "Disabled"}, false),
			},
			names.AttrRule: {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"default_retention": {
							Type:     schema.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									// OOS only supports COMPLIANCE mode.
									names.AttrMode: {
										Type:         schema.TypeString,
										Required:     true,
										ValidateFunc: validation.StringInSlice([]string{string(types.ObjectLockRetentionModeCompliance)}, false),
									},
									"days": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntBetween(1, 36500),
										ExactlyOneOf: []string{"rule.0.default_retention.0.days", "rule.0.default_retention.0.years"},
									},
									"years": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntBetween(1, 100),
										ExactlyOneOf: []string{"rule.0.default_retention.0.days", "rule.0.default_retention.0.years"},
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

func resourceBucketObjectLockConfigurationCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	cfg := expandOOSObjectLockConfiguration(d)

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketObjectLock(ctx, conn, bucket, cfg)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s) Object Lock Configuration: %s", bucket, err)
	}

	d.SetId(bucket)

	_, err = tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findObjectLockConfiguration(ctx, conn, bucket)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket Object Lock Configuration (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceBucketObjectLockConfigurationRead(ctx, d, meta)...)
}

func resourceBucketObjectLockConfigurationRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()

	objLockConfig, err := findObjectLockConfiguration(ctx, conn, bucket)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket Object Lock Configuration (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket Object Lock Configuration (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrBucket, bucket)
	d.Set("object_lock_enabled", objLockConfig.ObjectLockEnabled)
	if err := d.Set(names.AttrRule, flattenOOSObjectLockRule(objLockConfig.Rule)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting rule: %s", err)
	}

	return diags
}

func resourceBucketObjectLockConfigurationUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	cfg := expandOOSObjectLockConfiguration(d)

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketObjectLock(ctx, conn, bucket, cfg)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OOS Bucket Object Lock Configuration (%s): %s", d.Id(), err)
	}

	return append(diags, resourceBucketObjectLockConfigurationRead(ctx, d, meta)...)
}

func resourceBucketObjectLockConfigurationDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	// OOS provides a dedicated DELETE /{bucket}?object-lock that removes an
	// un-enabled configuration. Once object lock is enabled, OOS may reject
	// deletion with CanNotDeleteObjectLockConfiguration, which should surface
	// back to Terraform as an error rather than being treated as a no-op.
	err := oosDeleteBucketObjectLock(ctx, conn, bucket)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) || tfawserr.ErrCodeContains(err, errCodeObjectLockConfigurationNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket Object Lock Configuration (%s): %s", d.Id(), err)
	}

	return diags
}

// findObjectLockConfiguration retrieves the OOS object lock configuration,
// returning a NotFoundError when the bucket or configuration is absent.
func findObjectLockConfiguration(ctx context.Context, conn *s3.Client, bucket string) (*oosLockConfiguration, error) {
	output, err := oosGetBucketObjectLock(ctx, conn, bucket)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) || tfawserr.ErrCodeContains(err, errCodeObjectLockConfigurationNotFound) {
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

// expandOOSObjectLockConfiguration builds the OOS object lock configuration from
// the resource's top-level object_lock_enabled / rule attributes.
func expandOOSObjectLockConfiguration(d *schema.ResourceData) oosLockConfiguration {
	cfg := oosLockConfiguration{
		ObjectLockEnabled: d.Get("object_lock_enabled").(string),
	}

	if v, ok := d.GetOk(names.AttrRule); ok && len(v.([]any)) > 0 && v.([]any)[0] != nil {
		tfMap := v.([]any)[0].(map[string]any)

		if v, ok := tfMap["default_retention"].([]any); ok && len(v) > 0 && v[0] != nil {
			drMap := v[0].(map[string]any)
			dr := &oosLockDefaultRetention{}

			if v, ok := drMap[names.AttrMode].(string); ok && v != "" {
				dr.Mode = v
			}
			if v, ok := drMap["days"].(int); ok && v > 0 {
				dr.Days = aws.Int32(int32(v))
			}
			if v, ok := drMap["years"].(int); ok && v > 0 {
				dr.Years = aws.Int32(int32(v))
			}

			cfg.Rule = &oosLockRule{DefaultRetention: dr}
		}
	}

	return cfg
}

// flattenOOSObjectLockRule maps the OOS object lock rule into the rule block shape.
func flattenOOSObjectLockRule(rule *oosLockRule) []any {
	if rule == nil || rule.DefaultRetention == nil {
		return []any{}
	}

	dr := rule.DefaultRetention
	retention := map[string]any{
		names.AttrMode: dr.Mode,
		"days":         nil,
		"years":        nil,
	}
	if dr.Days != nil {
		retention["days"] = aws.ToInt32(dr.Days)
	}
	if dr.Years != nil {
		retention["years"] = aws.ToInt32(dr.Years)
	}

	return []any{
		map[string]any{
			"default_retention": []any{retention},
		},
	}
}
