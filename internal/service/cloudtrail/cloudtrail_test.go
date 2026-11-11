// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package cloudtrail_test

import (
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	acctest.RegisterServiceErrorCheckFunc(names.CloudTrailServiceID, testAccErrorCheckSkip)
}

// testAccErrorCheckSkip skips CloudTrail tests that have error messages indicating unsupported features.
func testAccErrorCheckSkip(t *testing.T) resource.ErrorCheckFunc {
	return acctest.ErrorCheckSkipMessagesContaining(t,
		"AccessDeniedException:",
	)
}
