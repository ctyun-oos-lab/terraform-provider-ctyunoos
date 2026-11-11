// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package sdkv2

import (
	"context"
	"fmt"
	"slices"
	"unique"

	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/interceptors"
	tftags "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tags"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types/option"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// tagsResourceCRUDInterceptor implements transparent tagging on CRUD operations for resources.
type tagsResourceCRUDInterceptor struct {
	interceptors.HTags
}

func resourceTransparentTagging(servicePackageResourceTags unique.Handle[inttypes.ServicePackageResourceTags]) crudInterceptor {
	return &tagsResourceCRUDInterceptor{
		HTags: interceptors.HTags(servicePackageResourceTags),
	}
}

func (r tagsResourceCRUDInterceptor) run(ctx context.Context, opts crudInterceptorOptions) diag.Diagnostics {
	c := opts.c
	var diags diag.Diagnostics

	if !r.Enabled() {
		return diags
	}

	sp, serviceName, resourceName, _, tagsInContext, ok := interceptors.InfoFromContext(ctx, c)
	if !ok {
		return diags
	}

	switch d, when, why := opts.d, opts.when, opts.why; when {
	case Before:
		switch why {
		case Create, Update:
			// Merge the resource's configured tags with any provider configured default_tags.
			tags := c.DefaultTagsConfig(ctx).MergeTags(tftags.New(ctx, d.Get(names.AttrTags).(map[string]any)))
			// Remove system tags.
			tags = tags.IgnoreSystem(sp.ServicePackageName())

			tagsInContext.TagsIn = option.Some(tags)

			if why == Create {
				break
			}

			// Without provider `default_tags`, the effective tags are exactly the
			// resource's configured `tags`, so drive updates off the `tags` change.
			if d.HasChange(names.AttrTags) {
				if identifier := r.GetIdentifierSDKv2(ctx, d); identifier != "" {
					o, n := d.GetChange(names.AttrTags)
					oldTags := tftags.New(ctx, o).IgnoreSystem(sp.ServicePackageName())
					newTags := tftags.New(ctx, n).IgnoreSystem(sp.ServicePackageName())

					if !oldTags.Equal(newTags) {
						if err := r.UpdateTags(ctx, sp, c, identifier, oldTags, newTags); err != nil {
							return sdkdiag.AppendErrorf(diags, "updating tags for %s %s (%s): %s", serviceName, resourceName, identifier, err)
						}
					}
					// TODO If the only change was to tags it would be nice to not call the resource's U handler.
				}
			}
		}
	case After:
		// Set tags and tags_all in state after CRU.
		// C & U handlers are assumed to tail call the R handler.
		switch why {
		case Read:
			// Will occur on a refresh when the resource does not exist in AWS and needs to be recreated, e.g. "_disappears" tests.
			if d.Id() == "" {
				return diags
			}

			fallthrough
		case Create, Update:
			// If the R handler didn't set tags, try and read them from the service API.
			if tagsInContext.TagsOut.IsNone() {
				// Some old resources may not have the required attribute set after Read:
				// https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/issues/31180
				if identifier := r.GetIdentifierSDKv2(ctx, d); identifier != "" {
					if err := r.ListTags(ctx, sp, c, identifier); err != nil {
						return sdkdiag.AppendErrorf(diags, "listing tags for %s %s (%s): %s", serviceName, resourceName, identifier, err)
					}
				}
			}

			// Remove any provider configured ignore_tags and system tags from those returned from the service API.
			tags := tagsInContext.TagsOut.UnwrapOrDefault().IgnoreSystem(sp.ServicePackageName()).IgnoreConfig(c.IgnoreTagsConfig(ctx))

			// The resource's configured tags can now include duplicate tags that have been configured on the provider.
			if err := d.Set(names.AttrTags, tags.ResolveDuplicates(ctx, c.DefaultTagsConfig(ctx), c.IgnoreTagsConfig(ctx), d, names.AttrTags, nil).Map()); err != nil {
				return sdkdiag.AppendErrorf(diags, "setting %s: %s", names.AttrTags, err)
			}
		}
	}

	return diags
}

// tagsDataSourceCRUDInterceptor implements transparent tagging on CRUD operations for data sources.
type tagsDataSourceCRUDInterceptor struct {
	interceptors.HTags
}

func dataSourceTransparentTagging(servicePackageResourceTags unique.Handle[inttypes.ServicePackageResourceTags]) crudInterceptor {
	return &tagsDataSourceCRUDInterceptor{
		HTags: interceptors.HTags(servicePackageResourceTags),
	}
}

func (r tagsDataSourceCRUDInterceptor) run(ctx context.Context, opts crudInterceptorOptions) diag.Diagnostics {
	c := opts.c
	var diags diag.Diagnostics

	if !r.Enabled() {
		return diags
	}

	sp, serviceName, resourceName, _, tagsInContext, ok := interceptors.InfoFromContext(ctx, c)
	if !ok {
		return diags
	}

	switch d, when, why := opts.d, opts.when, opts.why; when {
	case Before:
		switch why {
		case Read:
			// Get the data source's configured tags.
			tags := tftags.New(ctx, d.Get(names.AttrTags).(map[string]any))
			tagsInContext.TagsIn = option.Some(tags)
		}
	case After:
		// Set tags in state after R.
		switch why {
		case Read:
			// TODO: can this occur for a data source?
			if d.Id() == "" {
				return diags
			}

			// If the R handler didn't set tags, try and read them from the service API.
			if tagsInContext.TagsOut.IsNone() {
				// TODO: can this occur for a data source?
				// Some old resources may not have the required attribute set after Read:
				// https://github.com/ctyun-oos-lab/terraform-provider-ctyunoos/issues/31180
				if identifier := r.GetIdentifierSDKv2(ctx, d); identifier != "" {
					if err := r.ListTags(ctx, sp, c, identifier); err != nil {
						return sdkdiag.AppendErrorf(diags, "listing tags for %s %s (%s): %s", serviceName, resourceName, identifier, err)
					}
				}
			}

			// Remove any provider configured ignore_tags and system tags from those returned from the service API.
			tags := tagsInContext.TagsOut.UnwrapOrDefault().IgnoreSystem(sp.ServicePackageName()).IgnoreConfig(c.IgnoreTagsConfig(ctx))
			if err := d.Set(names.AttrTags, tags.Map()); err != nil {
				return sdkdiag.AppendErrorf(diags, "setting %s: %s", names.AttrTags, err)
			}
		}
	}

	return diags
}

func validateRequiredTags() customizeDiffInterceptor {
	return interceptorFunc1[*schema.ResourceDiff, error](func(ctx context.Context, opts customizeDiffInterceptorOptions) error {
		c := opts.c

		_, _, _, typeName, _, ok := interceptors.InfoFromContext(ctx, c)
		if !ok {
			return nil
		}

		policy := c.TagPolicyConfig(ctx)
		if policy == nil {
			return nil
		}
		reqTags, ok := policy.RequiredTags[typeName]
		if !ok {
			return nil
		}

		switch d, when, why := opts.d, opts.when, opts.why; when {
		case Before:
			switch why {
			case CustomizeDiff:
				isCreate := d.GetRawState().IsNull()
				hasTagsChange := d.HasChange(names.AttrTags)

				if !isCreate && !hasTagsChange {
					return nil
				}

				if !d.GetRawPlan().GetAttr(names.AttrTags).IsWhollyKnown() {
					return nil
				}

				cfgTags := tftags.New(ctx, d.Get(names.AttrTags).(map[string]any))
				allTags := c.DefaultTagsConfig(ctx).MergeTags(cfgTags)
				if allTags.ContainsAllKeys(reqTags) {
					return nil
				}

				missing := reqTags.Removed(allTags).Keys()
				slices.Sort(missing)
				summary := "Missing Required Tags"
				detail := fmt.Sprintf("An organizational tag policy requires the following tags for %s: %s", typeName, missing)

				// CustomizeDiff does not support diagnostics (only an error return)
				switch policy.Severity {
				case "warning":
					// Warning diagnostics are only logged
					tflog.Warn(ctx, "Required Tags Validation", map[string]any{
						"summary": summary,
						"detail":  detail,
					})
				default:
					// Error diagnostics merge summary and detail into a single message
					return fmt.Errorf("%s - %s", summary, detail)
				}
			}
		}

		return nil
	})
}
