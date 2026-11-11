// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"embed"
	"sync"

	"github.com/YakDriver/smarterr"
)

// Service-specific smarterr configuration is not currently used. Restore this
// directive when adding one; every active go:embed pattern must match a file.
// //go:embed service/*/smarterr.hcl
//
//go:embed service/smarterr.hcl
var SmarterrFS embed.FS

var registerSmarterrOnce sync.Once

// RegisterSmarterrFS registers the embedded Smarterr filesystem with the Smarterr package.
// This function should be called once during provider initialization.
//
// Note: go:embed can only embed files from the current directory or its subdirectories.
// Therefore, embedding must be performed from the `internal` package to ensure the
// correct global configuration file is included (`internal/service/smarterr.hcl`).
func RegisterSmarterrFS() {
	registerSmarterrOnce.Do(func() {
		smarterr.SetLogger(smarterr.TFLogLogger{})
		smarterr.SetFS(&smarterr.WrappedFS{FS: &SmarterrFS}, "internal")
	})
}
