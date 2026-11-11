// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package flex

import (
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// must panics if the provided Diagnostics has errors.
func must(diags diag.Diagnostics) {
	fwdiag.Must[any](nil, diags)
}
