// Copyright IBM Corp. 2014, 2026
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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_oos_bucket_policy", name="Bucket Policy")
func dataSourceBucketPolicy() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceBucketPolicyRead,

		Schema: map[string]*schema.Schema{
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(3, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9.-]+$`), "must contain only lowercase letters, numbers, periods, and hyphens"),
				),
			},
			names.AttrPolicy: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceBucketPolicyRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)

	policy, err := findBucketPolicy(ctx, conn, bucket)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OOS Bucket (%s) Policy: %s", bucket, err)
	}

	policy, err = structure.NormalizeJsonString(policy)
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	d.SetId(bucket)
	d.Set(names.AttrPolicy, policy)

	return diags
}
