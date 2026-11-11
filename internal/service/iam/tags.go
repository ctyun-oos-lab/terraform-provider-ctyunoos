// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build !generate

package iam

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types/option"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
)

// Custom IAM tag service update functions using the same format as generated code.

// userUpdateTags updates IAM user tags.
// The identifier is the user name.
func userUpdateTags(ctx context.Context, conn *iam.Client, identifier string, oldTagsMap, newTagsMap any) error {
	oldTags := tftags.New(ctx, oldTagsMap)
	newTags := tftags.New(ctx, newTagsMap)

	if removedTags := oldTags.Removed(newTags).IgnoreSystem(names.IAM); len(removedTags) > 0 {
		input := &iam.UntagUserInput{
			UserName: aws.String(identifier),
			TagKeys:  removedTags.Keys(),
		}

		_, err := tfresource.RetryWhenIsA[any, *awstypes.ConcurrentModificationException](ctx, propagationTimeout, func(ctx context.Context) (any, error) {
			return conn.UntagUser(ctx, input)
		})

		if err != nil {
			return fmt.Errorf("untagging resource (%s): %w", identifier, err)
		}
	}

	if updatedTags := oldTags.Updated(newTags).IgnoreSystem(names.IAM); len(updatedTags) > 0 {
		input := &iam.TagUserInput{
			UserName: aws.String(identifier),
			Tags:     svcTags(updatedTags),
		}

		_, err := tfresource.RetryWhenIsA[any, *awstypes.ConcurrentModificationException](ctx, propagationTimeout, func(ctx context.Context) (any, error) {
			return conn.TagUser(ctx, input)
		})

		if err != nil {
			return fmt.Errorf("tagging resource (%s): %w", identifier, err)
		}
	}

	return nil
}

func userKeyValueTags(ctx context.Context, conn *iam.Client, identifier string) (tftags.KeyValueTags, error) {
	tags, err := userTags(ctx, conn, identifier)
	if err != nil {
		return tftags.New(ctx, nil), fmt.Errorf("listing tags for resource (%s): %w", identifier, err)
	}

	return keyValueTags(ctx, tags), nil
}

// updateTags updates iam service tags.
// The identifier is typically the Amazon Resource Name (ARN), although
// it may also be a different identifier depending on the service.
func updateTags(ctx context.Context, conn *iam.Client, identifier, resourceType string, oldTagsMap, newTagsMap any) error {
	switch resourceType {
	case "User":
		return userUpdateTags(ctx, conn, identifier, oldTagsMap, newTagsMap)
	}

	return fmt.Errorf("unsupported resource type: %s", resourceType)
}

// ListTags lists iam service tags and set them in Context.
// It is called from outside this package.
func (p *servicePackage) ListTags(ctx context.Context, meta any, identifier, resourceType string) error {
	var (
		tags tftags.KeyValueTags
		err  error
	)
	switch resourceType {
	case "User":
		tags, err = userKeyValueTags(ctx, meta.(*conns.AWSClient).IAMClient(ctx), identifier)

	default:
		return nil
	}

	if err != nil {
		return err
	}

	if inContext, ok := tftags.FromContext(ctx); ok {
		inContext.TagsOut = option.Some(tags)
	}

	return nil
}

// UpdateTags updates iam service tags.
// It is called from outside this package.
func (p *servicePackage) UpdateTags(ctx context.Context, meta any, identifier, resourceType string, oldTags, newTags any) error {
	return updateTags(ctx, meta.(*conns.AWSClient).IAMClient(ctx), identifier, resourceType, oldTags, newTags)
}
