// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudtrail_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	tfcloudtrail "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/cloudtrail"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	sdkretry "github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const cloudTrailDataSourceTimeLayout = "2006-01-02T15:04:05.000Z"

const (
	cloudTrailLatestDeliveryStatusTimeout     = 2 * time.Minute
	cloudTrailLatestDeliveryErrorNoSuchBucket = "NoSuchBucket"
)

// TC-CT-TRAIL-DS-01 Basic Default Readback: verifies that the data source reads server-side default selectors and basic attributes.
func TestAccCloudTrailTrailDataSource_basicDefaultReadback(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"
	dataSourceName := "data.ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailDataSourceConfig(rName, "", false, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					testAccCheckTrailDataSourceState(ctx, t, dataSourceName),
					testAccCheckTrailEventSelectors(ctx, t, dataSourceName, types.ReadWriteTypeAll),
				),
			},
		},
	})
}

// TC-CT-TRAIL-DS-02 Explicit Readback And Refresh: verifies that the data source returns the latest attributes, selectors, and logging time after a trail update.
func TestAccCloudTrailTrailDataSource_explicitReadbackAndRefresh(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"
	dataSourceName := "data.ctyun_cloudtrail.test"
	var startLoggingTime time.Time

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailDataSourceConfig(rName, "initial-prefix", false, types.ReadWriteTypeWriteOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailDataSourceState(ctx, t, dataSourceName),
					testAccCheckTrailEventSelectors(ctx, t, dataSourceName, types.ReadWriteTypeWriteOnly),
				),
			},
			{
				Config: testAccCloudTrailDataSourceConfig(rName, "updated-prefix", true, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailDataSourceState(ctx, t, dataSourceName),
					testAccCheckTrailEventSelectors(ctx, t, dataSourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailDataSourceTimestamp(dataSourceName, "start_logging_time", &startLoggingTime, nil),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccCloudTrailDataSourceConfig(rName, "updated-prefix", false, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailDataSourceState(ctx, t, dataSourceName),
					testAccCheckTrailEventSelectors(ctx, t, dataSourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailDataSourceTimestamp(dataSourceName, "stop_logging_time", nil, &startLoggingTime),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-DS-03 Latest Delivery Readback: manually triggers NoSuchBucket and verifies readback of the latest delivery status.
func TestAccCloudTrailTrailDataSource_latestDeliveryStatus(t *testing.T) {
	t.Skip("manual local test: comment out this skip and trigger NoSuchBucket before running")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"
	dataSourceName := "data.ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailDataSourceConfig(rName, "", true, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					testAccWaitForTrailLatestDeliveryStatus(ctx, t, resourceName),
				),
			},
			{
				// Stop logging after the error has been observed so LatestDeliveryTime
				// no longer changes while the data source state is being compared.
				Config: testAccCloudTrailDataSourceConfig(rName, "", false, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailDataSourceState(ctx, t, dataSourceName),
					testAccCheckTrailDataSourceLatestDeliveryState(ctx, t, dataSourceName),
					testAccCheckTrailDataSourceTimestamp(dataSourceName, "latest_delivery_time", nil, nil),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-DS-04 Not Found: a nonexistent trail must return the provider's not-found diagnostic.
func TestAccCloudTrailTrailDataSource_notFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDataSourceRemovedFromState("data.ctyun_cloudtrail.test"),
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudTrailDataSourceConfig_notFound(rName),
				ExpectError: regexache.MustCompile(`(?i)reading CloudTrail Trail .*not found`),
			},
		},
	})
}

func testAccCheckTrailDataSourceState(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		if trailName == "" {
			return fmt.Errorf("CloudTrail data source (%s) has no name", dataSourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		trail, err := tfcloudtrail.FindTrailByName(ctx, conn, trailName)
		if err != nil {
			return fmt.Errorf("reading CloudTrail Trail (%s) for data source state: %w", trailName, err)
		}

		if got, want := rs.Primary.ID, aws.ToString(trail.TrailARN); got != want {
			return fmt.Errorf("expected data source ID %q from SDK response, got %q", want, got)
		}

		for key, want := range map[string]string{
			names.AttrARN:          aws.ToString(trail.TrailARN),
			names.AttrName:         aws.ToString(trail.Name),
			names.AttrS3BucketName: aws.ToString(trail.S3BucketName),
			names.AttrS3KeyPrefix:  aws.ToString(trail.S3KeyPrefix),
		} {
			got, ok := rs.Primary.Attributes[key]
			if !ok || got != want {
				return fmt.Errorf("expected data source state %s %q from SDK response, got %q", key, want, got)
			}
		}

		status, err := conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: aws.String(trailName)})
		if err != nil {
			return fmt.Errorf("getting CloudTrail Trail (%s) status for data source state: %w", trailName, err)
		}
		if status == nil {
			return fmt.Errorf("GetTrailStatus returned an empty response for CloudTrail Trail (%s)", trailName)
		}

		for key, want := range map[string]string{
			"enable_logging":     fmt.Sprintf("%t", aws.ToBool(status.IsLogging)),
			"start_logging_time": testAccCloudTrailDataSourceTimeString(status.StartLoggingTime),
			"stop_logging_time":  testAccCloudTrailDataSourceTimeString(status.StopLoggingTime),
		} {
			got, ok := rs.Primary.Attributes[key]
			if !ok || got != want {
				return fmt.Errorf("expected data source state %s %q from SDK response, got %q", key, want, got)
			}
		}

		selectors, err := conn.GetEventSelectors(ctx, &cloudtrail.GetEventSelectorsInput{TrailName: aws.String(trailName)})
		if err != nil {
			return fmt.Errorf("getting CloudTrail Trail (%s) event selectors for data source state: %w", trailName, err)
		}
		if selectors == nil {
			return fmt.Errorf("GetEventSelectors returned an empty response for CloudTrail Trail (%s)", trailName)
		}

		if want, got := fmt.Sprintf("%d", len(selectors.EventSelectors)), rs.Primary.Attributes["event_selector.#"]; got != want {
			return fmt.Errorf("expected data source state event_selector count %q from SDK response, got %q", want, got)
		}
		for i, selector := range selectors.EventSelectors {
			key := fmt.Sprintf("event_selector.%d.read_write_type", i)
			if want, got := string(selector.ReadWriteType), rs.Primary.Attributes[key]; got != want {
				return fmt.Errorf("expected data source state %s %q from SDK response, got %q", key, want, got)
			}
		}

		return nil
	}
}

func testAccCheckTrailDataSourceLatestDeliveryState(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		if trailName == "" {
			return fmt.Errorf("CloudTrail data source (%s) has no name", dataSourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		status, err := conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: aws.String(trailName)})
		if err != nil {
			return fmt.Errorf("getting CloudTrail Trail (%s) latest delivery status for data source state: %w", trailName, err)
		}
		if status == nil {
			return fmt.Errorf("GetTrailStatus returned an empty response for CloudTrail Trail (%s)", trailName)
		}
		if got, want := aws.ToString(status.LatestDeliveryError), cloudTrailLatestDeliveryErrorNoSuchBucket; got != want {
			return fmt.Errorf("expected CloudTrail Trail (%s) latest delivery error %q, got %q", trailName, want, got)
		}
		if status.LatestDeliveryTime == nil || status.LatestDeliveryTime.IsZero() {
			return fmt.Errorf("expected CloudTrail Trail (%s) latest delivery time to be set", trailName)
		}

		for key, want := range map[string]string{
			"latest_delivery_error": aws.ToString(status.LatestDeliveryError),
			"latest_delivery_time":  testAccCloudTrailDataSourceTimeString(status.LatestDeliveryTime),
		} {
			if got := rs.Primary.Attributes[key]; got != want {
				return fmt.Errorf("expected data source state %s %q from SDK response, got %q", key, want, got)
			}
		}

		return nil
	}
}

func testAccWaitForTrailLatestDeliveryStatus(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		if trailName == "" {
			return fmt.Errorf("CloudTrail Trail (%s) has no name", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		t.Logf("waiting up to %s for manually triggered latest delivery status on CloudTrail Trail %q", cloudTrailLatestDeliveryStatusTimeout, trailName)
		return sdkretry.RetryContext(ctx, cloudTrailLatestDeliveryStatusTimeout, func() *sdkretry.RetryError {
			status, err := conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: aws.String(trailName)})
			if err != nil {
				if tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
					return sdkretry.RetryableError(fmt.Errorf("waiting for CloudTrail Trail (%s) latest delivery status: %w", trailName, err))
				}

				return sdkretry.NonRetryableError(fmt.Errorf("reading CloudTrail Trail (%s) latest delivery status: %w", trailName, err))
			}
			if status == nil || aws.ToString(status.LatestDeliveryError) != cloudTrailLatestDeliveryErrorNoSuchBucket || status.LatestDeliveryTime == nil || status.LatestDeliveryTime.IsZero() {
				return sdkretry.RetryableError(fmt.Errorf("waiting for CloudTrail Trail (%s) latest delivery error %q and time", trailName, cloudTrailLatestDeliveryErrorNoSuchBucket))
			}

			return nil
		})
	}
}

func testAccCheckTrailDataSourceRemovedFromState(dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[dataSourceName]; ok {
			return fmt.Errorf("CloudTrail Trail data source (%s) is still present in state", dataSourceName)
		}

		return nil
	}
}

func testAccCloudTrailDataSourceTimeString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format(cloudTrailDataSourceTimeLayout)
}

func testAccCheckTrailDataSourceTimestamp(dataSourceName, attributeName string, captured, after *time.Time) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		value := rs.Primary.Attributes[attributeName]
		if value == "" {
			return fmt.Errorf("expected data source state %s to be set", attributeName)
		}

		parsed, err := time.Parse(cloudTrailDataSourceTimeLayout, value)
		if err != nil || parsed.UTC().Format(cloudTrailDataSourceTimeLayout) != value {
			return fmt.Errorf("expected data source state %s to use UTC millisecond format %q, got %q", attributeName, cloudTrailDataSourceTimeLayout, value)
		}

		if after != nil {
			if after.IsZero() {
				return fmt.Errorf("cannot compare %s because the earlier timestamp is empty", attributeName)
			}
			if !parsed.After(*after) {
				return fmt.Errorf("expected data source state %s %q to be after %q", attributeName, value, after.UTC().Format(cloudTrailDataSourceTimeLayout))
			}
		}

		if captured != nil {
			*captured = parsed
		}

		return nil
	}
}

func testAccCloudTrailDataSourceConfig(rName, s3KeyPrefix string, enableLogging bool, readWriteType types.ReadWriteType) string {
	prefixConfig := ""
	if s3KeyPrefix != "" {
		prefixConfig = fmt.Sprintf("  s3_key_prefix  = %q\n", s3KeyPrefix)
	}

	eventSelectorConfig := ""
	if readWriteType != "" {
		eventSelectorConfig = fmt.Sprintf(`
  event_selector {
    read_write_type = %q
  }
`, readWriteType)
	}

	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = %[2]t
%[3]s
%[4]s
}

data "ctyun_cloudtrail" "test" {
  name = ctyun_cloudtrail.test.name
}
`, rName, enableLogging, prefixConfig, eventSelectorConfig),
	)
}

func testAccCloudTrailDataSourceConfig_notFound(trailName string) string {
	return fmt.Sprintf(`
data "ctyun_cloudtrail" "test" {
  name = %[1]q
}
`, trailName)
}
