// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"log"
	"regexp"

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

const bucketSoftDeleteDefaultDays int32 = 7

// @SDKResource("ctyun_oos_bucket_soft_delete_configuration", name="Bucket Soft Delete")
// @IdentityAttribute("bucket")
// @Testing(hasNoPreExistingResource=true, disableRegionOverrideTest=true)
func resourceBucketSoftDelete() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketSoftDeleteCreate,
		ReadWithoutTimeout:   resourceBucketSoftDeleteRead,
		UpdateWithoutTimeout: resourceBucketSoftDeleteUpdate,
		DeleteWithoutTimeout: resourceBucketSoftDeleteDelete,

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
			// Retention window, in days, before deleted objects are permanently
			// removed. OOS accepts 1-15.
			"days": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntBetween(1, 15),
			},
		},
	}
}

func resourceBucketSoftDeleteCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	days := int32(d.Get("days").(int))

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketSoftDelete(ctx, conn, bucket, days)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OOS Bucket (%s) Soft Delete: %s", bucket, err)
	}

	d.SetId(bucket)

	return append(diags, resourceBucketSoftDeleteRead(ctx, d, meta)...)
}

func resourceBucketSoftDeleteRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()

	cfg, err := oosGetBucketSoftDelete(ctx, conn, bucket)

	// OOS always returns a configuration for an existing bucket, so a NotFound
	// here means the bucket itself is gone.
	if !d.IsNewResource() && (retry.NotFound(err) || tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket)) {
		log.Printf("[WARN] OOS Bucket Soft Delete (%s) not found, removing from state", bucket)
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) Soft Delete: %s", bucket, err)
	}

	d.Set(names.AttrBucket, bucket)
	if cfg != nil {
		d.Set("days", cfg.Days)
	}

	return diags
}

func resourceBucketSoftDeleteUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()
	days := int32(d.Get("days").(int))

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, bucketPropagationTimeout, func(ctx context.Context) (any, error) {
		return nil, oosPutBucketSoftDelete(ctx, conn, bucket, days)
	}, errCodeNoSuchBucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OOS Bucket (%s) Soft Delete: %s", bucket, err)
	}

	return append(diags, resourceBucketSoftDeleteRead(ctx, d, meta)...)
}

func resourceBucketSoftDeleteDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Id()
	unlock := lockBucketControlPlane(bucket)
	defer unlock()

	err := oosPutBucketSoftDelete(ctx, conn, bucket, bucketSoftDeleteDefaultDays)

	if tfawserr.ErrCodeEquals(err, errCodeNoSuchBucket) {
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OOS Bucket (%s) Soft Delete: %s", bucket, err)
	}

	return diags
}
