// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sweep

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/envvar"
	tfsync "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sync"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	ResourcePrefix = "tf-acc-test"
)

const defaultSweeperAssumeRoleDurationSeconds = 3600

// ServicePackages is set in TestMain in order to break an import cycle.
var ServicePackages []conns.ServicePackage

// sweeperClients is a shared cache of regional conns.AWSClient
// This prevents client re-initialization for every resource with no benefit.
var sweeperClients map[string]*conns.AWSClient = make(map[string]*conns.AWSClient)

// SharedRegionalSweepClient returns a common conns.AWSClient setup needed for the sweeper functions for a given Region.
func SharedRegionalSweepClient(ctx context.Context, region string) (*conns.AWSClient, error) {
	if client, ok := sweeperClients[region]; ok {
		return client, nil
	}

	_, _, err := envvar.RequireOneOf([]string{"CTYUNOOS_ACCESS_KEY"}, "OOS static credentials for running sweepers")
	if err != nil {
		return nil, err
	}

	_, err = envvar.Require("CTYUNOOS_SECRET_KEY", "static credentials value when using CTYUNOOS_ACCESS_KEY")
	if err != nil {
		return nil, err
	}

	meta := new(conns.AWSClient)
	servicePackageMap := make(map[string]conns.ServicePackage)
	for _, sp := range ServicePackages {
		servicePackageName := sp.ServicePackageName()
		servicePackageMap[servicePackageName] = sp
	}
	meta.SetServicePackages(ctx, servicePackageMap)

	endpoints := map[string]string{}
	if endpoint := names.EndpointForService(names.S3, region); endpoint != "" {
		endpoints[names.S3] = endpoint
	}
	if v := os.Getenv("CTYUNOOS_ENDPOINT"); v != "" {
		endpoints[names.S3] = v
	}
	if v := os.Getenv("CTYUNOOS_ENDPOINT_IAM"); v != "" {
		endpoints[names.IAM] = v
	}
	if v := os.Getenv("CTYUNOOS_ENDPOINT_TRAIL"); v != "" {
		endpoints[names.CloudTrail] = v
	}

	conf := &conns.Config{
		AccessKey:        os.Getenv("CTYUNOOS_ACCESS_KEY"),
		Endpoints:        endpoints,
		MaxRetries:       5,
		S3UsePathStyle:   true,
		SecretKey:        os.Getenv("CTYUNOOS_SECRET_KEY"),
		SuppressDebugLog: true,
		Token:            os.Getenv("CTYUNOOS_SESSION_TOKEN"),
	}

	if ar, ok, err := sweeperAssumeRoleFromEnv(); err != nil {
		return nil, err
	} else if ok {
		conf.AssumeRole = []awsbase.AssumeRole{ar}
	}

	client, diags := conf.ConfigureProvider(ctx, meta)

	if diags.HasError() {
		return nil, fmt.Errorf("getting AWS client: %#v", diags)
	}

	sweeperClients[region] = client

	return client, nil
}

func sweeperAssumeRoleFromEnv() (awsbase.AssumeRole, bool, error) {
	role := os.Getenv(envvar.AssumeRoleARN)
	if role == "" {
		return awsbase.AssumeRole{}, false, nil
	}

	ar := awsbase.AssumeRole{
		RoleARN:  role,
		Duration: time.Duration(defaultSweeperAssumeRoleDurationSeconds) * time.Second,
	}

	if v := os.Getenv(envvar.AssumeRoleDuration); v != "" {
		d, err := strconv.Atoi(v)
		if err != nil {
			return awsbase.AssumeRole{}, false, fmt.Errorf("environment variable %s: %w", envvar.AssumeRoleDuration, err)
		}
		ar.Duration = time.Duration(d) * time.Second
	}

	if v := os.Getenv(envvar.AssumeRoleExternalID); v != "" {
		ar.ExternalID = v
	}

	if v := os.Getenv(envvar.AssumeRoleSessionName); v != "" {
		ar.SessionName = v
	}

	return ar, true, nil
}

type Sweepable interface {
	Delete(ctx context.Context, optFns ...tfresource.OptionsFunc) error
}

func SweepOrchestrator(ctx context.Context, sweepables []Sweepable, optFns ...tfresource.OptionsFunc) error {
	if len(sweepables) == 0 {
		tflog.Info(ctx, "No resources to sweep")
	}

	var g tfsync.Group

	for _, sweepable := range sweepables {
		g.Go(ctx, func(ctx context.Context) error {
			return sweepable.Delete(ctx, optFns...)
		})
	}

	return g.Wait(ctx)
}

type SweeperFn func(ctx context.Context, client *conns.AWSClient) ([]Sweepable, error)
