// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package cloudtrail

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep/awsv2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func RegisterSweepers() {
	resource.AddTestSweepers("ctyun_cloudtrail", &resource.Sweeper{
		Name: "ctyun_cloudtrail",
		F:    sweepTrails,
	})
}

func sweepTrails(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}
	conn := client.CloudTrailClient(ctx)
	input := &cloudtrail.ListTrailsInput{}
	sweepResources := make([]sweep.Sweepable, 0)

	pages := cloudtrail.NewListTrailsPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping CloudTrail Trail sweep for %s: %s", region, err)
			return nil
		}

		if err != nil {
			return fmt.Errorf("error listing CloudTrail Trails (%s): %w", region, err)
		}

		for _, v := range page.Trails {
			arn := aws.ToString(v.TrailARN)

			_, err := findTrailByARN(ctx, conn, arn)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return fmt.Errorf("error reading CloudTrail Trail (%s): %w", arn, err)
			}

			r := resourceTrail()
			d := r.Data(nil)
			d.SetId(arn)

			sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))
		}
	}

	err = sweep.SweepOrchestrator(ctx, sweepResources)

	if err != nil {
		return fmt.Errorf("error sweeping CloudTrail Trails (%s): %w", region, err)
	}

	return nil
}
