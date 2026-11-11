// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// lifecycleDateValidator validates that a string Attribute's value is an
// ISO8601 timestamp in UTC midnight form, for example:
// 2002-10-11T00:00:00.000Z
type lifecycleDateValidator struct{}

func (validator lifecycleDateValidator) Description(_ context.Context) string {
	return "value must be an ISO8601 UTC midnight timestamp in the form 2002-10-11T00:00:00.000Z"
}

func (validator lifecycleDateValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

func (validator lifecycleDateValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			fmt.Sprintf("must be a valid ISO8601 timestamp: %s", err),
			value,
		))
		return
	}

	canonical := t.UTC().Format("2006-01-02T00:00:00.000Z")
	if value != canonical {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			fmt.Sprintf("must be a UTC midnight timestamp in the exact form %q", canonical),
			value,
		))
	}
}

// LifecycleDate returns a string validator which ensures that any configured
// attribute value:
//
//   - Is a valid ISO8601/RFC3339 timestamp.
//   - Represents UTC midnight.
//   - Uses the exact string form YYYY-MM-DDT00:00:00.000Z.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func LifecycleDate() validator.String {
	return lifecycleDateValidator{}
}
