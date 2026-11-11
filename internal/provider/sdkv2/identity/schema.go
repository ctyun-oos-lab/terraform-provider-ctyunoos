// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package identity

import (
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func NewIdentitySchema(identitySpec inttypes.Identity) map[string]*schema.Schema {
	identitySchema := make(map[string]*schema.Schema, len(identitySpec.Attributes))
	for _, attr := range identitySpec.Attributes {
		identitySchema[attr.Name()] = newIdentityAttribute(attr)
	}
	return identitySchema
}

func newIdentityAttribute(attribute inttypes.IdentityAttribute) *schema.Schema {
	attr := &schema.Schema{
		Type: schema.TypeString,
	}
	switch attribute.IdentityType() {
	case inttypes.BoolIdentityType:
		attr.Type = schema.TypeBool
	case inttypes.FloatIdentityType:
		attr.Type = schema.TypeFloat
	case inttypes.IntIdentityType:
		attr.Type = schema.TypeInt
	}

	if attribute.Required() {
		attr.RequiredForImport = true
	} else {
		attr.OptionalForImport = true
	}
	return attr
}
