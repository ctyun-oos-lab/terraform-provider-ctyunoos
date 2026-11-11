// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep/awsv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/sweep/sdk"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func RegisterSweepers() {
	resource.AddTestSweepers("ctyun_iam_group", &resource.Sweeper{
		Name: "ctyun_iam_group",
		F:    sweepGroups,
		Dependencies: []string{
			"ctyun_iam_user",
		},
	})

	resource.AddTestSweepers("ctyun_iam_policy", &resource.Sweeper{
		Name: "ctyun_iam_policy",
		F:    sweepPolicies,
		Dependencies: []string{
			"ctyun_iam_group",
			"ctyun_iam_role",
			"ctyun_iam_user",
		},
	})

	resource.AddTestSweepers("ctyun_iam_role", &resource.Sweeper{
		Name: "ctyun_iam_role",
		F:    sweepRoles,
	})

	awsv2.Register("ctyun_iam_saml_provider", sweepSAMLProviders)

	resource.AddTestSweepers("ctyun_iam_user", &resource.Sweeper{
		Name: "ctyun_iam_user",
		F:    sweepUsers,
		Dependencies: []string{
			"ctyun_iam_virtual_mfa_device",
		},
	})

	awsv2.Register("ctyun_iam_virtual_mfa_device", sweepVirtualMFADevice)
}

func sweepGroups(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	conn := client.IAMClient(ctx)
	var sweeperErrs *multierror.Error

	pages := iam.NewListGroupsPaginator(conn, &iam.ListGroupsInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMListGroupsResponse())
		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping IAM Group sweep for %s: %s", region, err)
			return sweeperErrs.ErrorOrNil()
		}
		if err != nil {
			sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("retrieving IAM Groups: %w", err))
			continue
		}

		for _, group := range page.Groups {
			name := aws.ToString(group.GroupName)
			if name == "Admin" || name == "TerraformAccTests" || !sweepNameFilter(name) {
				continue
			}

			log.Printf("[INFO] Deleting IAM Group: %s", name)
			if err := deleteGroupMembers(ctx, conn, name); err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("removing IAM Group (%s) members: %w", name, err))
				continue
			}
			if err := deleteGroupPolicyAttachments(ctx, conn, name); err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("deleting IAM Group (%s) policy attachments: %w", name, err))
				continue
			}

			_, err := conn.DeleteGroup(ctx, &iam.DeleteGroupInput{
				GroupName: group.GroupName,
			})
			if errs.IsA[*awstypes.NoSuchEntityException](err) {
				continue
			}
			if err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("deleting IAM Group (%s): %w", name, err))
				continue
			}
		}
	}

	return sweeperErrs.ErrorOrNil()
}

func deleteGroupMembers(ctx context.Context, conn *iam.Client, groupName string) error {
	output, err := conn.GetGroup(ctx, &iam.GetGroupInput{
		GroupName: aws.String(groupName),
	}, withOOSIAMGetGroupResponse())
	if errs.IsA[*awstypes.NoSuchEntityException](err) {
		return nil
	}
	if err != nil {
		return err
	}
	if output == nil {
		return nil
	}

	var errs []error
	for _, user := range output.Users {
		userName := aws.ToString(user.UserName)
		if err := removeUserFromGroup(ctx, conn, userName, groupName); err != nil && !tfresource.NotFound(err) {
			errs = append(errs, fmt.Errorf("removing IAM User (%s) from IAM Group (%s): %w", userName, groupName, err))
		}
	}

	return multierror.Append(nil, errs...).ErrorOrNil()
}

func sweepPolicies(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	conn := client.IAMClient(ctx)
	input := &iam.ListPoliciesInput{
		Scope: awstypes.PolicyScopeTypeLocal,
	}

	var sweepResources []sweep.Sweepable

	pages := iam.NewListPoliciesPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMListPoliciesResponse(""))
		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping IAM Policy sweep for %s: %s", region, err)
			return nil
		}
		if err != nil {
			return fmt.Errorf("retrieving IAM Policies: %w", err)
		}

		for _, policy := range page.Policies {
			arn := aws.ToString(policy.Arn)
			name := aws.ToString(policy.PolicyName)
			if !sweepNameFilter(name) {
				continue
			}
			if n := aws.ToInt32(policy.AttachmentCount); n > 0 {
				log.Printf("[INFO] Skipping IAM Policy %s: AttachmentCount=%d", arn, n)
				continue
			}

			r := resourcePolicy()
			d := r.Data(nil)
			d.SetId(arn)
			d.Set(names.AttrARN, arn)

			sweepResources = append(sweepResources, newPolicySweeper(r, d, client))
		}
	}

	if err := sweep.SweepOrchestrator(ctx, sweepResources); err != nil {
		return fmt.Errorf("sweeping IAM Policies (%s): %w", region, err)
	}

	return nil
}

type policySweeper struct {
	d         *schema.ResourceData
	sweepable sweep.Sweepable
}

func newPolicySweeper(resource *schema.Resource, d *schema.ResourceData, client *conns.AWSClient) *policySweeper {
	return &policySweeper{
		d:         d,
		sweepable: sdk.NewSweepResource(resource, d, client),
	}
}

func (ps policySweeper) Delete(ctx context.Context, optFns ...tfresource.OptionsFunc) error {
	if err := ps.sweepable.Delete(ctx, optFns...); err != nil {
		accessDenied := regexache.MustCompile(`AccessDenied: .+ with an explicit deny`)
		if accessDenied.MatchString(err.Error()) {
			log.Printf("[DEBUG] Skipping IAM Policy (%s): %s", ps.d.Id(), err)
			return nil
		}
		return err
	}
	return nil
}

func sweepRoles(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}
	conn := client.IAMClient(ctx)

	var roles []string
	pages := iam.NewListRolesPaginator(conn, &iam.ListRolesInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping IAM Role sweep for %s: %s", region, err)
			return nil
		}
		if err != nil {
			return fmt.Errorf("retrieving IAM Roles: %w", err)
		}

		for _, role := range page.Roles {
			roleName := aws.ToString(role.RoleName)
			if roleNameFilter(roleName) {
				roles = append(roles, roleName)
			} else {
				log.Printf("[INFO] Skipping IAM Role (%s): no match on allow-list", roleName)
			}
		}
	}

	var sweeperErrs *multierror.Error
	for _, roleName := range roles {
		log.Printf("[DEBUG] Deleting IAM Role (%s)", roleName)
		err := deleteRole(ctx, conn, roleName)
		if tfawserr.ErrCodeContains(err, "AccessDenied") {
			log.Printf("[WARN] Skipping IAM Role (%s): %s", roleName, err)
			continue
		}
		if err != nil {
			sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("deleting IAM Role (%s): %w", roleName, err))
			continue
		}
	}

	return sweeperErrs.ErrorOrNil()
}

func sweepSAMLProviders(ctx context.Context, client *conns.AWSClient) ([]sweep.Sweepable, error) {
	conn := client.IAMClient(ctx)
	var sweepResources []sweep.Sweepable

	out, err := conn.ListSAMLProviders(ctx, &iam.ListSAMLProvidersInput{})
	if err != nil {
		return sweepResources, err
	}

	for _, provider := range out.SAMLProviderList {
		arn := aws.ToString(provider.Arn)
		r := resourceSAMLProvider()
		d := r.Data(nil)
		d.SetId(arn)
		d.Set(names.AttrARN, arn)

		sweepResources = append(sweepResources, sdk.NewSweepResource(r, d, client))
	}

	return sweepResources, nil
}

func sweepUsers(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}
	conn := client.IAMClient(ctx)

	var sweepResources []sweep.Sweepable

	pages := iam.NewListUsersPaginator(conn, &iam.ListUsersInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx, withOOSIAMListUsersResponse())
		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping IAM User sweep for %s: %s", region, err)
			return nil
		}
		if err != nil {
			return err
		}

		for _, user := range page.Users {
			userName := aws.ToString(user.UserName)
			if !sweepNameFilter(userName) {
				continue
			}

			r := resourceUser()
			d := r.Data(nil)
			d.SetId(userName)
			d.Set(names.AttrName, userName)
			d.Set(names.AttrForceDestroy, true)

			sweepResources = append(sweepResources, sdk.NewSweepResource(r, d, client))
		}
	}

	if err := sweep.SweepOrchestrator(ctx, sweepResources); err != nil {
		return fmt.Errorf("sweeping IAM Users (%s): %w", region, err)
	}

	return nil
}

func roleNameFilter(name string) bool {
	if sweepNameFilter(name) {
		return true
	}

	prefixes := []string{
		names.AttrRole,
		"test",
		"tf-iam-role-replication",
		"tf-test-iam",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

func sweepNameFilter(name string) bool {
	standardNameRegexp := regexache.MustCompile(`tf[-_]acc[-_]test`)
	if standardNameRegexp.MatchString(name) {
		return true
	}

	randString10 := regexache.MustCompile(`^[0-9A-Za-z]{10}$`)
	if randString10.MatchString(name) {
		return true
	}

	prefixes := []string{
		"test-user",
		"test_user",
		"tf-acc",
		"tf_acc",
		acctestResourcePrefix(),
	}
	for _, prefix := range prefixes {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

func acctestResourcePrefix() string {
	return "tf-acc-test"
}

func sweepVirtualMFADevice(ctx context.Context, client *conns.AWSClient) ([]sweep.Sweepable, error) {
	conn := client.IAMClient(ctx)
	var sweepResources []sweep.Sweepable

	pages := iam.NewListVirtualMFADevicesPaginator(conn, &iam.ListVirtualMFADevicesInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, device := range page.VirtualMFADevices {
			serialNumber := aws.ToString(device.SerialNumber)
			if strings.Contains(serialNumber, "root-account-mfa-device") || !sweepNameFilter(serialNumber) {
				continue
			}

			r := resourceVirtualMFADevice()
			d := r.Data(nil)
			d.SetId(serialNumber)

			if user := device.User; user != nil {
				if userName := aws.ToString(user.UserName); userName != "" {
					d.Set(names.AttrUserName, userName)
				}
			}

			sweepResources = append(sweepResources, sdk.NewSweepResource(r, d, client))
		}
	}

	return sweepResources, nil
}
