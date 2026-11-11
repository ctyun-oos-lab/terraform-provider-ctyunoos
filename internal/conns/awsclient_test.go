// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package conns

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// TC-OOS-PROVIDER-46 S3 Resource Region Override verifies endpoint rewriting for a resource-level region.
// Expected: endpoint rewriting preserves connection details and does not construct an address when the endpoint is absent.
func TestUnitOOSProviderConfig_S3ResourceRegionEndpointPreservation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		service        string
		endpoint       string
		overrideRegion string
		expected       string
	}{
		{
			name:     "s3 custom port preserved",
			service:  "s3",
			endpoint: "https://oos-cn1.ctyunapi.cn:8080",
			expected: "https://oos-cn1.ctyunapi.cn:8080",
		},
		{
			name:           "s3 different region preserves port",
			service:        "s3",
			endpoint:       "https://oos-cn1.ctyunapi.cn:8080",
			overrideRegion: "cn4",
			expected:       "https://oos-cn4.ctyunapi.cn:8080",
		},
		{
			name:           "s3 missing endpoint remains empty",
			service:        "s3",
			overrideRegion: "shanghai",
			expected:       "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &AWSClient{
				endpoints: map[string]string{tc.service: tc.endpoint},
				awsConfig: &aws.Config{Region: "us-east-1"}, // lintignore:AWSAT003
			}
			ctx := NewResourceContext(t.Context(), tc.service, "", "", tc.overrideRegion)

			config := client.apiClientConfig(ctx, tc.service)
			if got, _ := config["endpoint"].(string); got != tc.expected {
				t.Errorf("endpoint = %q, expected %q", got, tc.expected)
			}
		})
	}
}
