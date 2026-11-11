// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/s3"
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

// @SDKResource("ctyun_oos_bucket_logging", name="Bucket Logging")
// @IdentityAttribute("bucket")
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceBucketLogging() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketLoggingCreate,
		ReadWithoutTimeout:   resourceBucketLoggingRead,
		UpdateWithoutTimeout: resourceBucketLoggingUpdate,
		DeleteWithoutTimeout: resourceBucketLoggingDelete,

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
			// OOS logging supports only TargetBucket / TargetPrefix (no grants,
			// no target_object_key_format).
			"target_bucket": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			"target_prefix": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceBucketLoggingCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	enabled := &oosLoggingEnabled{
		TargetBucket: d.Get("target_bucket").(string),
		TargetPrefix: d.Get("target_prefix").(string),
	}

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketLogging(ctx, conn, bucket, enabled)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s) Logging: %s", bucket, err)
	}

	d.SetId(bucket)

	_, err = tfresource.RetryWhenNotFound(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return findLoggingEnabled(ctx, conn, bucket)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for OOS Bucket Logging (%s) create: %s", d.Id(), err)
	}

	return append(diags, resourceBucketLoggingRead(ctx, d, meta)...)
}

func resourceBucketLoggingRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()

	loggingEnabled, err := findLoggingEnabled(ctx, conn, bucket)

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] OOS Bucket Logging (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket Logging (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrBucket, bucket)
	d.Set("target_bucket", loggingEnabled.TargetBucket)
	d.Set("target_prefix", loggingEnabled.TargetPrefix)

	return diags
}

func resourceBucketLoggingUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	enabled := &oosLoggingEnabled{
		TargetBucket: d.Get("target_bucket").(string),
		TargetPrefix: d.Get("target_prefix").(string),
	}

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketLogging(ctx, conn, bucket, enabled)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OOS Bucket Logging (%s): %s", d.Id(), err)
	}

	return append(diags, resourceBucketLoggingRead(ctx, d, meta)...)
}

func resourceBucketLoggingDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	// OOS has no DELETE for logging; a PUT with an empty BucketLoggingStatus
	// disables it. oosPutBucketLogging sends an empty body when enabled is nil.
	err := oosPutBucketLogging(ctx, conn, bucket, nil)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket Logging (%s): %s", d.Id(), err)
	}

	return diags
}

// findLoggingEnabled retrieves the OOS bucket logging configuration. OOS returns
// no LoggingEnabled element when logging is disabled, which oosGetBucketLogging
// surfaces as a nil result; this resource treats "disabled" as not found.
func findLoggingEnabled(ctx context.Context, conn *s3.Client, bucket string) (*oosLoggingEnabled, error) {
	output, err := oosGetBucketLogging(ctx, conn, bucket)

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
