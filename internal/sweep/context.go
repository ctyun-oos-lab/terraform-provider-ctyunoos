// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sweep

import (
	"context"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep/internal/log"
	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
)

func Context(region string) context.Context {
	ctx := context.Background()

	ctx = tfsdklog.ContextWithStandardLogging(ctx, "sweep")

	ctx = log.Logger(ctx, "sweeper", region)

	return ctx
}
