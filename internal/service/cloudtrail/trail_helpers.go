// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package cloudtrail

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/enum"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func eventSelectorResourceSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Computed: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"read_write_type": {
					Type:             schema.TypeString,
					Optional:         true,
					Computed:         true,
					ValidateDiagFunc: enum.Validate[types.ReadWriteType](),
				},
			},
		},
	}
}

func eventSelectorDataSourceSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Computed: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"read_write_type": {
					Type:     schema.TypeString,
					Computed: true,
				},
			},
		},
	}
}

func trailNameFromARN(arn string) (string, error) {
	const suffix = ":trail/"
	if i := strings.LastIndex(arn, suffix); i >= 0 && i+len(suffix) < len(arn) {
		return arn[i+len(suffix):], nil
	}

	return "", fmt.Errorf("invalid CloudTrail trail ARN (%s)", arn)
}

func trailAPIName(d *schema.ResourceData) (string, error) {
	if v := d.Get(names.AttrName).(string); v != "" {
		return v, nil
	}

	return trailNameFromARN(d.Id())
}

func findTrailByARN(ctx context.Context, conn *cloudtrail.Client, arn string) (*types.Trail, error) {
	name, err := trailNameFromARN(arn)
	if err != nil {
		return nil, err
	}

	return findTrailByName(ctx, conn, name)
}

func findTrailByName(ctx context.Context, conn *cloudtrail.Client, name string) (*types.Trail, error) {
	input := &cloudtrail.DescribeTrailsInput{
		TrailNameList: []string{name},
	}

	return findTrail(ctx, conn, input)
}

func findTrail(ctx context.Context, conn *cloudtrail.Client, input *cloudtrail.DescribeTrailsInput) (*types.Trail, error) {
	output, err := findTrails(ctx, conn, input)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findTrails(ctx context.Context, conn *cloudtrail.Client, input *cloudtrail.DescribeTrailsInput) ([]types.Trail, error) {
	output, err := conn.DescribeTrails(ctx, input)

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	trails := output.TrailList

	if input != nil && len(input.TrailNameList) > 0 {
		trails = filterTrailsByName(trails, input.TrailNameList)
	}

	if len(trails) == 0 {
		return nil, tfresource.NewEmptyResultError()
	}

	return trails, nil
}

func filterTrailsByName(trails []types.Trail, names []string) []types.Trail {
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[name] = struct{}{}
	}

	filtered := make([]types.Trail, 0, len(trails))
	for _, trail := range trails {
		if _, ok := want[aws.ToString(trail.Name)]; ok {
			filtered = append(filtered, trail)
		}
	}

	return filtered
}

func readTrailEventSelectors(ctx context.Context, conn *cloudtrail.Client, trailName string) ([]map[string]any, error) {
	output, err := conn.GetEventSelectors(ctx, &cloudtrail.GetEventSelectorsInput{
		TrailName: aws.String(trailName),
	})

	if err != nil {
		return nil, err
	}

	return flattenEventSelector(output.EventSelectors), nil
}

func readTrailLoggingStatus(ctx context.Context, conn *cloudtrail.Client, trailName string) (*cloudtrail.GetTrailStatusOutput, error) {
	return conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
		Name: aws.String(trailName),
	})
}

// flattenTimeString formats a timestamp for Terraform output.
// OOS returns epoch milliseconds; convertOOSCloudTrailTimestampsJSON in service_package.go
// rewrites API responses to epoch seconds before the AWS SDK deserializes them.
func flattenTimeString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func flattenEventSelector(configured []types.EventSelector) []map[string]any {
	eventSelectors := make([]map[string]any, 0, len(configured))

	for _, raw := range configured {
		eventSelectors = append(eventSelectors, map[string]any{
			"read_write_type": string(raw.ReadWriteType),
		})
	}

	return eventSelectors
}

func expandEventSelector(configured []any) []types.EventSelector {
	eventSelectors := make([]types.EventSelector, 0, len(configured))

	for _, raw := range configured {
		data := raw.(map[string]any)

		eventSelectors = append(eventSelectors, types.EventSelector{
			ReadWriteType: types.ReadWriteType(data["read_write_type"].(string)),
		})
	}

	return eventSelectors
}

func setEventSelectors(ctx context.Context, conn *cloudtrail.Client, trailName string, configured []any) error {
	eventSelectors := expandEventSelector(configured)
	if len(eventSelectors) == 0 {
		eventSelectors = append(eventSelectors, types.EventSelector{
			ReadWriteType: types.ReadWriteTypeAll,
		})
	}

	_, err := conn.PutEventSelectors(ctx, &cloudtrail.PutEventSelectorsInput{
		TrailName:      aws.String(trailName),
		EventSelectors: eventSelectors,
	})

	if err != nil {
		return fmt.Errorf("setting CloudTrail Trail (%s) event selectors: %w", trailName, err)
	}

	return nil
}

func setLogging(ctx context.Context, conn *cloudtrail.Client, trailName string, enabled bool) error {
	if enabled {
		_, err := conn.StartLogging(ctx, &cloudtrail.StartLoggingInput{
			Name: aws.String(trailName),
		})

		if err != nil {
			return fmt.Errorf("starting CloudTrail Trail (%s) logging: %w", trailName, err)
		}
	} else {
		_, err := conn.StopLogging(ctx, &cloudtrail.StopLoggingInput{
			Name: aws.String(trailName),
		})

		if err != nil {
			return fmt.Errorf("stopping CloudTrail Trail (%s) logging: %w", trailName, err)
		}
	}

	return nil
}
