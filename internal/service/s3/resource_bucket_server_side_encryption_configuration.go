// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// @SDKResource("ctyun_oos_bucket_server_side_encryption_configuration", name="Bucket Server Side Encryption Configuration")
// @IdentityAttribute("bucket")
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
// @Testing(plannableImportAction="NoOp")
// @Testing(checkDestroyNoop=true)
func resourceBucketServerSideEncryptionConfiguration() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketServerSideEncryptionConfigurationCreate,
		ReadWithoutTimeout:   resourceBucketServerSideEncryptionConfigurationRead,
		UpdateWithoutTimeout: resourceBucketServerSideEncryptionConfigurationUpdate,
		// OOS does not allow removing bucket server-side encryption once configured,
		// so destroying this resource only removes it from Terraform state.
		DeleteWithoutTimeout: schema.NoopContext,

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
			// OOS supports a single rule carrying only
			// apply_server_side_encryption_by_default.sse_algorithm = AES256
			// (no KMS key, no bucket key, no blocked encryption types).
			names.AttrRule: {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"apply_server_side_encryption_by_default": {
							Type:     schema.TypeList,
							MaxItems: 1,
							Required: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"sse_algorithm": {
										Type:         schema.TypeString,
										Required:     true,
										ValidateFunc: validation.StringInSlice([]string{string(awstypes.ServerSideEncryptionAes256)}, false),
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

func resourceBucketServerSideEncryptionConfigurationCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	algorithm := expandSSEAlgorithm(d.Get(names.AttrRule).([]any))

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketEncryption(ctx, conn, bucket, algorithm)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s) Server-side Encryption Configuration: %s", bucket, err)
	}

	d.SetId(bucket)

	_, err = tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findServerSideEncryptionConfiguration(ctx, conn, bucket)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket Server-side Encryption Configuration (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceBucketServerSideEncryptionConfigurationRead(ctx, d, meta)...)
}

func resourceBucketServerSideEncryptionConfigurationRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()

	sse, err := findServerSideEncryptionConfiguration(ctx, conn, bucket)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket Server-side Encryption Configuration (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket Server-side Encryption Configuration (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrBucket, bucket)
	if err := d.Set(names.AttrRule, flattenSSERules(sse)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting rule: %s", err)
	}

	return diags
}

func resourceBucketServerSideEncryptionConfigurationUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	algorithm := expandSSEAlgorithm(d.Get(names.AttrRule).([]any))

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketEncryption(ctx, conn, bucket, algorithm)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OOS Bucket Server-side Encryption Configuration (%s): %s", d.Id(), err)
	}

	return append(diags, resourceBucketServerSideEncryptionConfigurationRead(ctx, d, meta)...)
}

// findServerSideEncryptionConfiguration retrieves the OOS server-side encryption
// configuration, returning a NotFoundError when none is set (OOS returns a
// 404 NoSuchServerSideEncryptionRule, or an empty body).
func findServerSideEncryptionConfiguration(ctx context.Context, conn *s3.Client, bucket string) (*oosServerSideEncryptionConfiguration, error) {
	output, err := oosGetBucketEncryption(ctx, conn, bucket)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket, errCodeNoSuchServerSideEncryptionRule) || tfawserr.ErrCodeContains(err, errCodeServerSideEncryptionConfigurationNotFound) {
		return nil, &retry.NotFoundError{LastError: err}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Rule == nil || output.Rule.ApplyServerSideEncryptionByDefault == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}

// expandSSEAlgorithm pulls the single SSEAlgorithm out of the rule block.
func expandSSEAlgorithm(l []any) string {
	if len(l) == 0 || l[0] == nil {
		return ""
	}

	tfMap, ok := l[0].(map[string]any)
	if !ok {
		return ""
	}

	def, ok := tfMap["apply_server_side_encryption_by_default"].([]any)
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

// flattenSSERules maps the OOS server-side encryption configuration into the
// rule block shape.
func flattenSSERules(cfg *oosServerSideEncryptionConfiguration) []any {
	if cfg == nil || cfg.Rule == nil || cfg.Rule.ApplyServerSideEncryptionByDefault == nil {
		return []any{}
	}

	return []any{
		map[string]any{
			"apply_server_side_encryption_by_default": []any{
				map[string]any{
					"sse_algorithm": cfg.Rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm,
				},
			},
		},
	}
}
