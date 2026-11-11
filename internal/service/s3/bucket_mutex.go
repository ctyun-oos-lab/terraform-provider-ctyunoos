// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"fmt"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
)

const bucketControlPlaneMutexPrefix = "oos-s3-bucket"

func bucketControlPlaneMutexKey(bucket string) string {
	return fmt.Sprintf("%s/%s", bucketControlPlaneMutexPrefix, bucket)
}

func lockBucketControlPlane(bucket string) func() {
	key := bucketControlPlaneMutexKey(bucket)
	conns.GlobalMutexKV.Lock(key)

	return func() {
		conns.GlobalMutexKV.Unlock(key)
	}
}
