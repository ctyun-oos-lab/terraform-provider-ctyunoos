// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package stringvalidator

import (
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/framework/validators/internal"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func WarnExactlyOneOf(expressions ...path.Expression) validator.String {
	return internal.ExactlyOneOfValidator{
		PathExpressions: expressions,
	}
}
