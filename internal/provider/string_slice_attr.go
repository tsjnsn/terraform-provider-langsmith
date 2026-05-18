// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringSliceToAttrValues(ss []string) []attr.Value {
	out := make([]attr.Value, len(ss))
	for i, s := range ss {
		out[i] = types.StringValue(s)
	}
	return out
}
