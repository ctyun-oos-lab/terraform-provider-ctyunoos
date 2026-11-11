// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sweep_test

import (
	"context"
	"testing"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	sweep.ServicePackages = servicePackages(ctx)

	registerSweepers()

	resource.TestMain(m)
}
