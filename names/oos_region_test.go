// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package names

import "testing"

func TestEndpointForService(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		TestName string
		Service  string
		Region   string
		Expected string
	}{
		{TestName: "s3 cn1", Service: "s3", Region: "cn1", Expected: "https://oos-cn1.ctyunapi.cn"},
		{TestName: "s3 override region", Service: "s3", Region: "cn4", Expected: "https://oos-cn4.ctyunapi.cn"},
		{TestName: "iam cn3", Service: "iam", Region: "cn3", Expected: "https://oos-cn3-iam.ctyunapi.cn"},
		{TestName: "cloudtrail cn2", Service: "cloudtrail", Region: "cn2", Expected: "https://oos-cn2-cloudtrail.ctyunapi.cn"},
		{TestName: "empty region returns empty", Service: "s3", Region: "", Expected: ""},
		{TestName: "unknown service returns empty", Service: "doesnotexist", Region: "cn1", Expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			t.Parallel()

			if got := EndpointForService(testCase.Service, testCase.Region); got != testCase.Expected {
				t.Errorf("got %q, expected %q", got, testCase.Expected)
			}
		})
	}
}

func TestRegionForEndpoint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		TestName string
		Service  string
		Endpoint string
		Expected string
	}{
		{TestName: "s3 exact", Service: "s3", Endpoint: "https://oos-cn4.ctyunapi.cn", Expected: "cn4"},
		{TestName: "iam exact", Service: "iam", Endpoint: "https://oos-cn4-iam.ctyunapi.cn", Expected: "cn4"},
		{TestName: "cloudtrail exact", Service: "cloudtrail", Endpoint: "https://oos-cn2-cloudtrail.ctyunapi.cn", Expected: "cn2"},
		{TestName: "http scheme tolerated", Service: "s3", Endpoint: "http://oos-cn4.ctyunapi.cn", Expected: "cn4"},
		{TestName: "trailing slash tolerated", Service: "s3", Endpoint: "https://oos-cn4.ctyunapi.cn/", Expected: "cn4"},
		{TestName: "multi-segment region name", Service: "s3", Endpoint: "https://oos-cn-east-1.ctyunapi.cn", Expected: "cn-east-1"},
		{TestName: "non-matching custom host", Service: "s3", Endpoint: "https://my-private-oos.example.com", Expected: ""},
		{TestName: "host with extra dotted segment", Service: "s3", Endpoint: "https://oos-cn4.extra.ctyunapi.cn", Expected: ""},
		{TestName: "empty endpoint", Service: "s3", Endpoint: "", Expected: ""},
		{TestName: "unknown service", Service: "doesnotexist", Endpoint: "https://oos-cn4.ctyunapi.cn", Expected: ""},
		{TestName: "s3 with port", Service: "s3", Endpoint: "https://oos-cn4.ctyunapi.cn:8443", Expected: "cn4"},
		{TestName: "iam with port", Service: "iam", Endpoint: "https://oos-cn4-iam.ctyunapi.cn:8080", Expected: "cn4"},
		{TestName: "host with port and trailing slash", Service: "s3", Endpoint: "https://oos-cn4.ctyunapi.cn:8443/", Expected: "cn4"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			t.Parallel()

			if got := RegionForEndpoint(testCase.Service, testCase.Endpoint); got != testCase.Expected {
				t.Errorf("got %q, expected %q", got, testCase.Expected)
			}
		})
	}
}

func TestRequiresConfiguredEndpoint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		TestName string
		Service  string
		Expected bool
	}{
		{TestName: "s3 is an OOS service", Service: "s3", Expected: true},
		{TestName: "iam is an OOS service", Service: "iam", Expected: true},
		{TestName: "cloudtrail is an OOS service", Service: "cloudtrail", Expected: true},
		{TestName: "unknown service", Service: "doesnotexist", Expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			t.Parallel()

			if got := RequiresConfiguredEndpoint(testCase.Service); got != testCase.Expected {
				t.Errorf("got %t, expected %t", got, testCase.Expected)
			}
		})
	}
}

func TestRebaseEndpointRegion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		TestName string
		Service  string
		Endpoint string
		Region   string
		Expected string
	}{
		{
			TestName: "s3 preserves scheme and port",
			Service:  "s3",
			Endpoint: "http://oos-shanghai.ctyunapi.cn:8080",
			Region:   "beijing",
			Expected: "http://oos-beijing.ctyunapi.cn:8080",
		},
		{
			TestName: "s3 https no port",
			Service:  "s3",
			Endpoint: "https://oos-shanghai.ctyunapi.cn",
			Region:   "cn4",
			Expected: "https://oos-cn4.ctyunapi.cn",
		},
		{
			TestName: "s3 preserves trailing path",
			Service:  "s3",
			Endpoint: "http://oos-shanghai.ctyunapi.cn:8080/",
			Region:   "beijing",
			Expected: "http://oos-beijing.ctyunapi.cn:8080/",
		},
		{
			TestName: "s3 short to hyphenated region",
			Service:  "s3",
			Endpoint: "https://oos-cn1.ctyunapi.cn",
			Region:   "cn-north-1",
			Expected: "https://oos-cn-north-1.ctyunapi.cn",
		},
		{
			TestName: "iam preserves the iam suffix segment",
			Service:  "iam",
			Endpoint: "http://oos-beijing-iam.ctyunapi.cn:8080",
			Region:   "shanghai",
			Expected: "http://oos-shanghai-iam.ctyunapi.cn:8080",
		},
		{TestName: "empty endpoint returns empty", Service: "s3", Endpoint: "", Region: "beijing", Expected: ""},
		{TestName: "empty region returns empty", Service: "s3", Endpoint: "https://oos-shanghai.ctyunapi.cn", Region: "", Expected: ""},
		{TestName: "unknown service returns empty", Service: "doesnotexist", Endpoint: "https://oos-shanghai.ctyunapi.cn", Region: "beijing", Expected: ""},
		{TestName: "non-matching host returns empty", Service: "s3", Endpoint: "https://my-private-oos.example.com", Region: "beijing", Expected: ""},
		{TestName: "host with extra dotted segment returns empty", Service: "s3", Endpoint: "https://oos-shanghai.extra.ctyunapi.cn", Region: "beijing", Expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			t.Parallel()

			if got := RebaseEndpointRegion(testCase.Service, testCase.Endpoint, testCase.Region); got != testCase.Expected {
				t.Errorf("got %q, expected %q", got, testCase.Expected)
			}
		})
	}
}
