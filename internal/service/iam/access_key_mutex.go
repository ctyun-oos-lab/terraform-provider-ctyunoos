// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"strings"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
)

const accessKeysMutexPrefix = "oos-iam-access-keys"

func accessKeysMutexKey(username string) string {
	return accessKeysMutexPrefix + "/" + strings.ToLower(username)
}

func lockAccessKeys(username string) func() {
	key := accessKeysMutexKey(username)
	conns.GlobalMutexKV.Lock(key)

	return func() {
		conns.GlobalMutexKV.Unlock(key)
	}
}
