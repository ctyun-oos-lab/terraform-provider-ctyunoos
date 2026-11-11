// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package framework

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/fwdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/framework/resourceattribute"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestResourceSetRegionInStateInterceptor_Read(t *testing.T) {
	t.Parallel()

	const name = "example"

	region := "a_region"

	ctx := context.Background()
	client := mockClient{region: region}
	icpt := resourceSetRegionInStateInterceptor{}

	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrName:   schema.StringAttribute{Required: true},
			names.AttrRegion: resourceattribute.Region(),
		},
	}

	tests := map[string]struct {
		startState tfsdk.State
		expectSet  bool
	}{
		"when state is present then region is set": {
			startState: stateFromSchema(ctx, s, map[string]string{"name": name}),
			expectSet:  true,
		},
		"when state is null then it remains null": {
			startState: tfsdk.State{
				Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), nil),
				Schema: s,
			},
			expectSet: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			req := resource.ReadRequest{State: tc.startState}
			resp := resource.ReadResponse{State: tc.startState}

			icpt.read(ctx, interceptorOptions[resource.ReadRequest, resource.ReadResponse]{
				c:        client,
				request:  &req,
				response: &resp,
				when:     After,
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diags: %s", resp.Diagnostics)
			}

			if tc.expectSet {
				got := getStateAttributeValue(ctx, t, resp.State, path.Root("region"))
				if got != region {
					t.Errorf("expected region %q, got %q", region, got)
				}
			} else {
				if !resp.State.Raw.IsNull() {
					t.Errorf("expected State.Raw to stay null, got %#v", resp.State.Raw)
				}
			}
		})
	}
}

func TestDataSourceSetRegionInStateInterceptor_Read(t *testing.T) {
	t.Parallel()

	const (
		name                = "example"
		configuredRegion    = "sdk-default-region"
		effectiveRealRegion = "oos-real-region"
	)

	ctx := context.Background()
	client := mockClientWithEffectiveRegion{region: configuredRegion, effectiveRealRegion: effectiveRealRegion}
	icpt := dataSourceSetRegionInStateInterceptor{}

	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrName:   schema.StringAttribute{Required: true},
			names.AttrRegion: resourceattribute.Region(),
		},
	}

	tests := map[string]struct {
		startState tfsdk.State
		expectSet  bool
	}{
		"when state is present then effective real region is set": {
			startState: stateFromSchema(ctx, s, map[string]string{"name": name}),
			expectSet:  true,
		},
		"when state is null then it remains null": {
			startState: tfsdk.State{
				Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), nil),
				Schema: s,
			},
			expectSet: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			req := datasource.ReadRequest{}
			resp := datasource.ReadResponse{State: tc.startState}

			icpt.read(ctx, interceptorOptions[datasource.ReadRequest, datasource.ReadResponse]{
				c:        client,
				request:  &req,
				response: &resp,
				when:     After,
			})
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diags: %s", resp.Diagnostics)
			}

			if tc.expectSet {
				got := getStateAttributeValue(ctx, t, resp.State, path.Root("region"))
				if got != effectiveRealRegion {
					t.Errorf("expected region %q, got %q", effectiveRealRegion, got)
				}
			} else {
				if !resp.State.Raw.IsNull() {
					t.Errorf("expected State.Raw to stay null, got %#v", resp.State.Raw)
				}
			}
		})
	}
}

type mockClientWithEffectiveRegion struct {
	region              string
	effectiveRealRegion string
}

func (c mockClientWithEffectiveRegion) AccountID(_ context.Context) string {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) Region(_ context.Context) string {
	return c.region
}

func (c mockClientWithEffectiveRegion) EffectiveRealRegion(_ context.Context) string {
	return c.effectiveRealRegion
}

func (c mockClientWithEffectiveRegion) DefaultTagsConfig(context.Context) *tags.DefaultConfig {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) IgnoreTagsConfig(context.Context) *tags.IgnoreConfig {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) Partition(context.Context) string {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) ServicePackage(context.Context, string) conns.ServicePackage {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) TagPolicyConfig(context.Context) *tags.TagPolicyConfig {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) ValidateInContextRegionInPartition(context.Context) error {
	panic("not implemented")
}

func (c mockClientWithEffectiveRegion) AwsConfig(context.Context) aws.Config {
	panic("not implemented")
}

func getStateAttributeValue(ctx context.Context, t *testing.T, st tfsdk.State, p path.Path) string {
	t.Helper()

	var v types.String
	if diags := st.GetAttribute(ctx, p, &v); diags.HasError() {
		t.Fatalf("unexpected error getting State attribute %q: %s", p, fwdiag.DiagnosticsError(diags))
	}
	return v.ValueString()
}
