// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework"
	fwtypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/smerr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// @FrameworkDataSource("ctyun_iam_virtual_mfa_devices", name="Virtual MFA Devices")
func newVirtualMFADevicesDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	return &virtualMFADevicesDataSource{}, nil
}

type virtualMFADevicesDataSource struct {
	framework.DataSourceWithModel[virtualMFADevicesDataSourceModel]
}

func (d *virtualMFADevicesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"virtual_mfa_devices": framework.DataSourceComputedListOfObjectAttribute[virtualMFADeviceDataSourceModel](ctx),
		},
	}
}

func (d *virtualMFADevicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	conn := d.Meta().IAMClient(ctx)

	var data virtualMFADevicesDataSourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Config.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	var input iam.ListVirtualMFADevicesInput
	devices, err := findVirtualMFADevicesForDataSource(ctx, conn, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err)
		return
	}

	data.ID = types.StringValue(d.Meta().EffectiveRealRegion(ctx))
	data.VirtualMFADevices, resp.Diagnostics = fwtypes.NewListNestedObjectValueOfValueSlice(ctx, flattenVirtualMFADevices(ctx, devices))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &data))
}

func findVirtualMFADevicesForDataSource(ctx context.Context, conn *iam.Client, input *iam.ListVirtualMFADevicesInput) ([]awstypes.VirtualMFADevice, error) {
	var output []awstypes.VirtualMFADevice

	paginator := iam.NewListVirtualMFADevicesPaginator(conn, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		output = append(output, page.VirtualMFADevices...)
	}

	return output, nil
}

func flattenVirtualMFADevices(ctx context.Context, devices []awstypes.VirtualMFADevice) []virtualMFADeviceDataSourceModel {
	output := make([]virtualMFADeviceDataSourceModel, 0, len(devices))

	for _, device := range devices {
		user := flattenVirtualMFADeviceUser(ctx, device.User)
		output = append(output, virtualMFADeviceDataSourceModel{
			EnableDate:   types.StringValue(policyTimeString(device.EnableDate)),
			SerialNumber: types.StringValue(aws.ToString(device.SerialNumber)),
			User:         user,
		})
	}

	return output
}

func flattenVirtualMFADeviceUser(ctx context.Context, user *awstypes.User) fwtypes.ObjectValueOf[virtualMFADeviceUserDataSourceModel] {
	model := virtualMFADeviceUserDataSourceModel{
		ARN:              types.StringValue(""),
		CreateDate:       types.StringValue(""),
		PasswordLastUsed: types.StringValue(""),
		UserID:           types.StringValue(""),
		UserName:         types.StringValue(""),
	}

	if user != nil {
		model = virtualMFADeviceUserDataSourceModel{
			ARN:              types.StringValue(aws.ToString(user.Arn)),
			CreateDate:       types.StringValue(policyTimeString(user.CreateDate)),
			PasswordLastUsed: types.StringValue(policyTimeString(user.PasswordLastUsed)),
			UserID:           types.StringValue(aws.ToString(user.UserId)),
			UserName:         types.StringValue(aws.ToString(user.UserName)),
		}
	}

	return fwtypes.NewObjectValueOfMust(ctx, &model)
}

type virtualMFADevicesDataSourceModel struct {
	ID                types.String                                                     `tfsdk:"id"`
	VirtualMFADevices fwtypes.ListNestedObjectValueOf[virtualMFADeviceDataSourceModel] `tfsdk:"virtual_mfa_devices"`
}

type virtualMFADeviceDataSourceModel struct {
	EnableDate   types.String                                               `tfsdk:"enable_date"`
	SerialNumber types.String                                               `tfsdk:"serial_number"`
	User         fwtypes.ObjectValueOf[virtualMFADeviceUserDataSourceModel] `tfsdk:"user"`
}

type virtualMFADeviceUserDataSourceModel struct {
	ARN              types.String `tfsdk:"arn"`
	CreateDate       types.String `tfsdk:"create_date"`
	PasswordLastUsed types.String `tfsdk:"password_last_used"`
	UserID           types.String `tfsdk:"user_id"`
	UserName         types.String `tfsdk:"user_name"`
}
