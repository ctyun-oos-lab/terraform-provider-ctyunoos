// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudtrail

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// @SDKDataSource("ctyun_cloudtrail", name="Trail")
func dataSourceTrail() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceTrailRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enable_logging": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"event_selector": eventSelectorDataSourceSchema(),
			"latest_delivery_error": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"latest_delivery_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrName: {
				Type:     schema.TypeString,
				Required: true,
			},
			names.AttrS3BucketName: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrS3KeyPrefix: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"start_logging_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"stop_logging_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceTrailRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CloudTrailClient(ctx)

	name := d.Get(names.AttrName).(string)

	trail, err := findTrailByName(ctx, conn, name)

	if retry.NotFound(err) {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s): not found", name)
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s): %s", name, err)
	}

	arn := aws.ToString(trail.TrailARN)
	d.SetId(arn)
	d.Set(names.AttrARN, arn)
	d.Set(names.AttrName, trail.Name)
	d.Set(names.AttrS3BucketName, trail.S3BucketName)
	d.Set(names.AttrS3KeyPrefix, trail.S3KeyPrefix)

	status, err := readTrailLoggingStatus(ctx, conn, name)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s) status: %s", name, err)
	}

	d.Set("enable_logging", status.IsLogging)
	d.Set("latest_delivery_error", aws.ToString(status.LatestDeliveryError))
	d.Set("latest_delivery_time", flattenTimeString(status.LatestDeliveryTime))
	d.Set("start_logging_time", flattenTimeString(status.StartLoggingTime))
	d.Set("stop_logging_time", flattenTimeString(status.StopLoggingTime))

	eventSelectors, err := readTrailEventSelectors(ctx, conn, name)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading CloudTrail Trail (%s) event selectors: %s", name, err)
	}

	if err := d.Set("event_selector", eventSelectors); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting event_selector: %s", err)
	}

	return diags
}
