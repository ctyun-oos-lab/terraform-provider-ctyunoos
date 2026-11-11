// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package sweep

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/envvar"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
)

// TC-OOS-PROVIDER-36 AssumeRole Sweep Env Var verifies expansion of AssumeRole environment variables for sweepers and the default duration.
func TestUnitOOSSweeper_AssumeRoleEnvVar(t *testing.T) {
	testCases := []struct {
		name    string
		env     map[string]string
		want    awsbase.AssumeRole
		wantSet bool
		wantErr string
	}{
		{
			name: "role ARN uses default duration",
			env: map[string]string{
				envvar.AssumeRoleARN: "arn:ctyun:iam::32fefj64y54gc:role/SweeperRole",
			},
			want: awsbase.AssumeRole{
				RoleARN:  "arn:ctyun:iam::32fefj64y54gc:role/SweeperRole",
				Duration: time.Hour,
			},
			wantSet: true,
		},
		{
			name: "all environment variables override defaults",
			env: map[string]string{
				envvar.AssumeRoleARN:         "arn:ctyun:iam::32fefj64y54gc:role/SweeperRole",
				envvar.AssumeRoleDuration:    "1800",
				envvar.AssumeRoleExternalID:  "sweeper-external-id",
				envvar.AssumeRoleSessionName: "sweeper-session",
			},
			want: awsbase.AssumeRole{
				RoleARN:     "arn:ctyun:iam::32fefj64y54gc:role/SweeperRole",
				Duration:    30 * time.Minute,
				ExternalID:  "sweeper-external-id",
				SessionName: "sweeper-session",
			},
			wantSet: true,
		},
		{
			name: "role ARN omitted",
		},
		{
			name: "invalid duration",
			env: map[string]string{
				envvar.AssumeRoleARN:      "arn:ctyun:iam::32fefj64y54gc:role/SweeperRole",
				envvar.AssumeRoleDuration: "invalid",
			},
			wantErr: envvar.AssumeRoleDuration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envvar.AssumeRoleARN, "")
			t.Setenv(envvar.AssumeRoleDuration, "")
			t.Setenv(envvar.AssumeRoleExternalID, "")
			t.Setenv(envvar.AssumeRoleSessionName, "")
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			got, gotSet, err := sweeperAssumeRoleFromEnv()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("expanding sweeper AssumeRole environment variables: %v", err)
			}
			if gotSet != tc.wantSet {
				t.Fatalf("configured = %t, want %t", gotSet, tc.wantSet)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("AssumeRole = %#v, want %#v", got, tc.want)
			}
		})
	}
}
