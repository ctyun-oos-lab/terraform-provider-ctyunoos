// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
)

func testProbeListUsersWithStaticCredentials(ctx context.Context, meta *conns.AWSClient, endpoint, accessKeyID, secretAccessKey string) error {
	client, err := iamClientWithStaticCredentials(ctx, meta, endpoint, accessKeyID, secretAccessKey)
	if err != nil {
		return err
	}

	_, err = client.ListUsers(ctx, &iam.ListUsersInput{}, withOOSIAMListUsersResponse())
	return err
}

func testProbeGetUserWithStaticCredentials(ctx context.Context, meta *conns.AWSClient, endpoint, accessKeyID, secretAccessKey string) error {
	client, err := iamClientWithStaticCredentials(ctx, meta, endpoint, accessKeyID, secretAccessKey)
	if err != nil {
		return err
	}

	_, err = client.GetUser(ctx, &iam.GetUserInput{})
	return err
}

func iamClientWithStaticCredentials(ctx context.Context, meta *conns.AWSClient, endpoint, accessKeyID, secretAccessKey string) (*iam.Client, error) {
	cfg := meta.AwsConfig(ctx)
	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	return (&servicePackage{}).NewClient(ctx, map[string]any{
		"aws_sdkv2_config": &cfg,
		names.AttrEndpoint: endpoint,
		names.AttrRegion:   meta.Region(ctx),
	})
}
