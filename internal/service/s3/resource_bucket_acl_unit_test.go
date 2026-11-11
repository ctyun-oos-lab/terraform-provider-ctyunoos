// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
)

func TestUnitCannedACLFromGrants(t *testing.T) {
	t.Parallel()

	const allUsersURI = "http://acs.amazonaws.com/groups/global/AllUsers"

	testCases := []struct {
		name string
		acl  *awss3.GetBucketAclOutput
		want string
	}{
		{
			name: "nil acl",
			want: "private",
		},
		{
			name: "empty grants",
			acl:  &awss3.GetBucketAclOutput{},
			want: "private",
		},
		{
			name: "non all users grant",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{{
					Permission: types.PermissionRead,
					Grantee: &types.Grantee{
						Type: types.TypeCanonicalUser,
						URI:  aws.String("http://example.com/not-all-users"),
					},
				}},
			},
			want: "private",
		},
		{
			name: "all users with empty permission",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{{
					Grantee: &types.Grantee{
						Type: types.TypeGroup,
						URI:  aws.String(allUsersURI),
					},
				}},
			},
			want: "private",
		},
		{
			name: "all users read",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{{
					Permission: types.PermissionRead,
					Grantee: &types.Grantee{
						Type: types.TypeGroup,
						URI:  aws.String(allUsersURI),
					},
				}},
			},
			want: "public-read",
		},
		{
			name: "all users full control",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{{
					Permission: types.PermissionFullControl,
					Grantee: &types.Grantee{
						Type: types.TypeGroup,
						URI:  aws.String(allUsersURI),
					},
				}},
			},
			want: "public-read-write",
		},
		{
			name: "unrelated grant before all users read",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{
					{
						Permission: types.PermissionRead,
						Grantee: &types.Grantee{
							Type: types.TypeCanonicalUser,
							URI:  aws.String("http://example.com/not-all-users"),
						},
					},
					{
						Permission: types.PermissionRead,
						Grantee: &types.Grantee{
							Type: types.TypeGroup,
							URI:  aws.String(allUsersURI),
						},
					},
				},
			},
			want: "public-read",
		},
		{
			name: "all users full control before unrelated grant",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{
					{
						Permission: types.PermissionFullControl,
						Grantee: &types.Grantee{
							Type: types.TypeGroup,
							URI:  aws.String(allUsersURI),
						},
					},
					{
						Permission: types.PermissionRead,
						Grantee: &types.Grantee{
							Type: types.TypeCanonicalUser,
							URI:  aws.String("http://example.com/not-all-users"),
						},
					},
				},
			},
			want: "public-read-write",
		},
		{
			name: "all users unknown permission",
			acl: &awss3.GetBucketAclOutput{
				Grants: []types.Grant{{
					Permission: types.PermissionWrite,
					Grantee: &types.Grantee{
						Type: types.TypeGroup,
						URI:  aws.String(allUsersURI),
					},
				}},
			},
			want: "private",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := cannedACLFromGrants(testCase.acl); got != testCase.want {
				t.Fatalf("cannedACLFromGrants() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestUnitFlattenBucketACLPolicy(t *testing.T) {
	t.Parallel()

	const allUsersURI = "http://acs.amazonaws.com/groups/global/AllUsers"

	t.Run("nil acl", func(t *testing.T) {
		t.Parallel()

		if got := flattenBucketACLPolicy(nil); len(got) != 0 {
			t.Fatalf("flattenBucketACLPolicy(nil) length = %d, want 0", len(got))
		}
	})

	t.Run("full policy", func(t *testing.T) {
		t.Parallel()

		acl := &awss3.GetBucketAclOutput{
			Owner: &types.Owner{
				DisplayName: aws.String("owner-display"),
				ID:          aws.String("owner-id"),
			},
			Grants: []types.Grant{{
				Permission: types.PermissionFullControl,
				Grantee: &types.Grantee{
					Type: types.TypeGroup,
					URI:  aws.String(allUsersURI),
				},
			}},
		}

		got := flattenBucketACLPolicy(acl)
		if len(got) != 1 {
			t.Fatalf("flattenBucketACLPolicy() length = %d, want 1", len(got))
		}

		policy, ok := got[0].(map[string]any)
		if !ok {
			t.Fatalf("flattenBucketACLPolicy()[0] type = %T, want map[string]any", got[0])
		}

		owners, ok := policy["owner"].([]any)
		if !ok || len(owners) != 1 {
			t.Fatalf("owner = %#v, want single owner element", policy["owner"])
		}
		owner, ok := owners[0].(map[string]any)
		if !ok {
			t.Fatalf("owner[0] type = %T, want map[string]any", owners[0])
		}
		if got, want := owner[names.AttrDisplayName], "owner-display"; got != want {
			t.Fatalf("owner display_name = %#v, want %#v", got, want)
		}
		if got, want := owner[names.AttrID], "owner-id"; got != want {
			t.Fatalf("owner id = %#v, want %#v", got, want)
		}

		grants, ok := policy["grant"].([]any)
		if !ok || len(grants) != 1 {
			t.Fatalf("grant = %#v, want single grant element", policy["grant"])
		}
		grant, ok := grants[0].(map[string]any)
		if !ok {
			t.Fatalf("grant[0] type = %T, want map[string]any", grants[0])
		}
		if got, want := grant["permission"], "FULL_CONTROL"; got != want {
			t.Fatalf("permission = %#v, want %#v", got, want)
		}

		grantees, ok := grant["grantee"].([]any)
		if !ok || len(grantees) != 1 {
			t.Fatalf("grantee = %#v, want single grantee element", grant["grantee"])
		}
		grantee, ok := grantees[0].(map[string]any)
		if !ok {
			t.Fatalf("grantee[0] type = %T, want map[string]any", grantees[0])
		}
		if got, want := grantee[names.AttrType], "Group"; got != want {
			t.Fatalf("grantee type = %#v, want %#v", got, want)
		}
		if got, want := grantee[names.AttrURI], allUsersURI; got != want {
			t.Fatalf("grantee uri = %#v, want %#v", got, want)
		}
	})
}

func TestUnitFlattenBucketACLGrants(t *testing.T) {
	t.Parallel()

	if got := flattenBucketACLGrants(nil); len(got) != 0 {
		t.Fatalf("flattenBucketACLGrants(nil) length = %d, want 0", len(got))
	}
}

func TestUnitFlattenBucketACLGrantee(t *testing.T) {
	t.Parallel()

	t.Run("nil grantee", func(t *testing.T) {
		t.Parallel()

		if got := flattenBucketACLGrantee(nil); len(got) != 0 {
			t.Fatalf("flattenBucketACLGrantee(nil) length = %d, want 0", len(got))
		}
	})

	t.Run("maps type and uri", func(t *testing.T) {
		t.Parallel()

		got := flattenBucketACLGrantee(&types.Grantee{
			Type: types.TypeGroup,
			URI:  aws.String("http://acs.amazonaws.com/groups/global/AllUsers"),
		})
		if len(got) != 1 {
			t.Fatalf("flattenBucketACLGrantee() length = %d, want 1", len(got))
		}
		grantee, ok := got[0].(map[string]any)
		if !ok {
			t.Fatalf("flattenBucketACLGrantee()[0] type = %T, want map[string]any", got[0])
		}
		if got, want := grantee[names.AttrType], "Group"; got != want {
			t.Fatalf("type = %#v, want %#v", got, want)
		}
		if got, want := grantee[names.AttrURI], "http://acs.amazonaws.com/groups/global/AllUsers"; got != want {
			t.Fatalf("uri = %#v, want %#v", got, want)
		}
	})
}

func TestUnitFlattenBucketACLOwner(t *testing.T) {
	t.Parallel()

	t.Run("nil owner", func(t *testing.T) {
		t.Parallel()

		if got := flattenBucketACLOwner(nil); len(got) != 0 {
			t.Fatalf("flattenBucketACLOwner(nil) length = %d, want 0", len(got))
		}
	})

	t.Run("maps display name and id", func(t *testing.T) {
		t.Parallel()

		got := flattenBucketACLOwner(&types.Owner{
			DisplayName: aws.String("owner-display"),
			ID:          aws.String("owner-id"),
		})
		if len(got) != 1 {
			t.Fatalf("flattenBucketACLOwner() length = %d, want 1", len(got))
		}
		owner, ok := got[0].(map[string]any)
		if !ok {
			t.Fatalf("flattenBucketACLOwner()[0] type = %T, want map[string]any", got[0])
		}
		if got, want := owner[names.AttrDisplayName], "owner-display"; got != want {
			t.Fatalf("display_name = %#v, want %#v", got, want)
		}
		if got, want := owner[names.AttrID], "owner-id"; got != want {
			t.Fatalf("id = %#v, want %#v", got, want)
		}
	})
}
