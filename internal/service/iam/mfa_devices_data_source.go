// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKDataSource("ctyun_iam_mfa_devices", name="MFA Devices")
func dataSourceMFADevices() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceMFADevicesRead,

		Schema: map[string]*schema.Schema{
			"mfa_devices": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enable_date": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"serial_number": {
							Type:     schema.TypeString,
							Computed: true,
						},
						names.AttrUserName: {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			names.AttrUserName: {
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 64),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
		},
	}
}

func dataSourceMFADevicesRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	input := &iam.ListMFADevicesInput{}
	if v, ok := d.GetOk(names.AttrUserName); ok {
		input.UserName = aws.String(v.(string))
	}

	devices, err := findMFADevicesForDataSource(ctx, conn, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM MFA Devices: %s", err)
	}

	d.SetId(meta.(*conns.AWSClient).EffectiveRealRegion(ctx))

	if err := d.Set("mfa_devices", flattenMFADevicesForDataSource(devices)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting mfa_devices: %s", err)
	}

	return diags
}

func findMFADevicesForDataSource(ctx context.Context, conn *iam.Client, input *iam.ListMFADevicesInput) ([]awstypes.MFADevice, error) {
	var output []awstypes.MFADevice

	paginator := iam.NewListMFADevicesPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		output = append(output, page.MFADevices...)
	}

	return output, nil
}

func flattenMFADevicesForDataSource(devices []awstypes.MFADevice) []map[string]any {
	if len(devices) == 0 {
		return []map[string]any{}
	}

	output := make([]map[string]any, 0, len(devices))
	for i := range devices {
		output = append(output, flattenMFADeviceForDataSource(&devices[i]))
	}
	return output
}

func flattenMFADeviceForDataSource(device *awstypes.MFADevice) map[string]any {
	if device == nil {
		return map[string]any{}
	}

	return map[string]any{
		"enable_date":      policyTimeString(device.EnableDate),
		"serial_number":    aws.ToString(device.SerialNumber),
		names.AttrUserName: aws.ToString(device.UserName),
	}
}
