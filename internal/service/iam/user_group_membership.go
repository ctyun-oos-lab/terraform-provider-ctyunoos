// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_user_group_membership", name="User Group Membership")
// @IdentityAttribute("user")
// @IdentityAttribute("group")
// @IdAttrFormat("{user}/{group}")
// @ImportIDHandler("userGroupMembershipImportID")
// @Testing(hasNoPreExistingResource=true)
func resourceUserGroupMembership() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceUserGroupMembershipCreate,
		ReadWithoutTimeout:   resourceUserGroupMembershipRead,
		DeleteWithoutTimeout: resourceUserGroupMembershipDelete,

		Schema: map[string]*schema.Schema{
			"user": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 64),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
			"group": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 128),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
		},
	}
}

func resourceUserGroupMembershipCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	user := d.Get("user").(string)
	group := d.Get("group").(string)

	if err := addUserToGroup(ctx, conn, user, group); err != nil {
		return sdkdiag.AppendErrorf(diags, "assigning IAM User Group Membership (%s/%s): %s", user, group, err)
	}

	d.SetId(createUserGroupMembershipImportID(d))

	return append(diags, resourceUserGroupMembershipRead(ctx, d, meta)...)
}

func resourceUserGroupMembershipRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	user := d.Get("user").(string)
	group := d.Get("group").(string)
	id := userGroupMembershipID(user, group)

	_, err := tfresource.RetryWhenNewResourceNotFound(ctx, propagationTimeout, func(ctx context.Context) (any, error) {
		return findGroupForUserByTwoPartKey(ctx, conn, user, group)
	}, d.IsNewResource())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM User Group Membership (%s) not found, removing from state", id)
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM User Group Membership (%s): %s", id, err)
	}

	d.SetId(id)

	return diags
}

func resourceUserGroupMembershipDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)
	user := d.Get("user").(string)
	group := d.Get("group").(string)

	if err := removeUserFromGroup(ctx, conn, user, group); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM User Group Membership (%s/%s): %s", user, group, err)
	}
	return diags
}

func addUserToGroup(ctx context.Context, conn *iam.Client, user, group string) error {
	_, err := conn.AddUserToGroup(ctx, &iam.AddUserToGroupInput{
		UserName:  aws.String(user),
		GroupName: aws.String(group),
	})
	if err != nil {
		return fmt.Errorf("adding User (%s) to Group (%s): %w", user, group, err)
	}
	return nil
}

func removeUserFromGroup(ctx context.Context, conn *iam.Client, user, group string) error {
	_, err := conn.RemoveUserFromGroup(ctx, &iam.RemoveUserFromGroupInput{
		UserName:  aws.String(user),
		GroupName: aws.String(group),
	})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("removing IAM User (%s) from group (%s): %w", user, group, err)
	}

	return nil
}

func findGroupForUserByTwoPartKey(ctx context.Context, conn *iam.Client, user, group string) (*awstypes.Group, error) {
	input := &iam.ListGroupsForUserInput{
		UserName: aws.String(user),
	}

	var output []awstypes.Group
	err := listGroupsForUserPages(ctx, conn, input, func(page *iam.ListGroupsForUserOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, v := range page.Groups {
			if aws.ToString(v.GroupName) == group {
				output = append(output, v)
			}
		}

		return !lastPage
	})

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil, &retry.NotFoundError{LastError: err}
	}

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func createUserGroupMembershipImportID(d *schema.ResourceData) string {
	return (userGroupMembershipImportID{}).Create(d)
}

type userGroupMembershipImportID struct{}

func (v userGroupMembershipImportID) Create(d *schema.ResourceData) string {
	return v.create(d.Get("user").(string), d.Get("group").(string))
}

func (userGroupMembershipImportID) create(user, group string) string {
	return userGroupMembershipID(user, group)
}

func (userGroupMembershipImportID) Parse(id string) (string, map[string]any, error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", nil, fmt.Errorf("unexpected format for Import ID (%q), expected <user-name>/<group-name>", id)
	}

	result := map[string]any{
		"user":  parts[0],
		"group": parts[1],
	}
	return id, result, nil
}

func userGroupMembershipID(user, group string) string {
	return fmt.Sprintf("%s/%s", user, group)
}
