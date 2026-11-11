// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"regexp"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_oos_bucket_soft_delete_configuration", name="Bucket Soft Delete")
func dataSourceBucketSoftDelete() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceBucketSoftDeleteRead,

		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			// Retention window, in days, before deleted objects are permanently
			// removed. OOS always returns a value (a server-side default when none
			// was set).
			"days": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceBucketSoftDeleteRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)

	cfg, err := oosGetBucketSoftDelete(ctx, conn, bucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) Soft Delete: %s", bucket, err)
	}

	return dataSourceBucketSoftDeleteSetState(d, bucket, cfg)
}

func dataSourceBucketSoftDeleteSetState(d *schema.ResourceData, bucket string, cfg *oosSoftDeleteConfiguration) diag.Diagnostics {
	var diags diag.Diagnostics

	if cfg == nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) Soft Delete: empty configuration response", bucket)
	}

	d.SetId(bucket)
	d.Set("days", cfg.Days)

	return diags
}
