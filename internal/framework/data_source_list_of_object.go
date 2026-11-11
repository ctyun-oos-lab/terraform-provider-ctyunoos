// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package framework

import (
	"context"

	fwtypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DataSourceComputedListOfObjectAttribute returns a new schema.ListAttribute for objects of the specified type.
// The list is Computed-only.
func DataSourceComputedListOfObjectAttribute[T any](ctx context.Context) schema.ListAttribute {
	return schema.ListAttribute{
		CustomType: fwtypes.NewListNestedObjectTypeOf[T](ctx),
		Computed:   true,
		ElementType: types.ObjectType{
			AttrTypes: fwtypes.AttributeTypesMust[T](ctx),
		},
	}
}
