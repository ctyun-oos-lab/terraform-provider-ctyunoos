// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package validators_test

import (
	"context"
	"testing"

	fwvalidators "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/validators"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAWSAccountIDValidator(t *testing.T) { // nosemgrep:ci.aws-in-func-name
	t.Parallel()

	type testCase struct {
		val                 types.String
		expectedDiagnostics diag.Diagnostics
	}
	tests := map[string]testCase{
		"unknown String": {
			val: types.StringUnknown(),
		},
		"null String": {
			val: types.StringNull(),
		},
		"invalid String": {
			val: types.StringValue("test-value"),
			expectedDiagnostics: diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(
					path.Root("test"),
					"Invalid Attribute Value",
					`Attribute test value must be a valid CTYUN account ID, got: test-value`,
				),
			},
		},
		"valid CTYUN account ID": {
			val: types.StringValue("10rc2arpn6306"),
		},
		"13 digits CTYUN account ID": {
			val: types.StringValue("1234567890123"),
		},
		"AWS account ID is too short": {
			val: types.StringValue("123456789012"),
			expectedDiagnostics: diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(
					path.Root("test"),
					"Invalid Attribute Value",
					`Attribute test value must be a valid CTYUN account ID, got: 123456789012`,
				),
			},
		},
		"too short CTYUN account ID": {
			val: types.StringValue("12345678901"),
			expectedDiagnostics: diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(
					path.Root("test"),
					"Invalid Attribute Value",
					`Attribute test value must be a valid CTYUN account ID, got: 12345678901`,
				),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			request := validator.StringRequest{
				Path:           path.Root("test"),
				PathExpression: path.MatchRoot("test"),
				ConfigValue:    test.val,
			}
			response := validator.StringResponse{}
			fwvalidators.AWSAccountID().ValidateString(ctx, request, &response)

			if diff := cmp.Diff(response.Diagnostics, test.expectedDiagnostics); diff != "" {
				t.Errorf("unexpected diagnostics difference: %s", diff)
			}
		})
	}
}
