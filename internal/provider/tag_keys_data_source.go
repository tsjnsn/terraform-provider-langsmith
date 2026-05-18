// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &TagKeysDataSource{}

func NewTagKeysDataSource() datasource.DataSource {
	return &TagKeysDataSource{}
}

// TagKeysDataSource lists tag keys for the current LangSmith workspace.
type TagKeysDataSource struct {
	client *client.Client
}

type TagKeysDataSourceModel struct {
	TagKeys types.List `tfsdk:"tag_keys"`
}

var tagKeyRecordObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"key":         types.StringType,
	"description": types.StringType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}}

func (d *TagKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_keys"
}

func (d *TagKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all tag keys defined in the current LangSmith workspace.\n\n" +
			"This data source calls `GET /api/v1/workspaces/current/tag-keys` and returns the full array returned by the API. " +
			"The LangSmith API does not expose query parameters for filtering, pagination, or sorting on this endpoint; " +
			"Terraform receives every tag key the server returns for the workspace in a single response.",
		Attributes: map[string]schema.Attribute{
			"tag_keys": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the tag key.",
						},
						"key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The tag key name.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "A description of the tag key, when present.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Creation timestamp.",
						},
						"updated_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Last update timestamp.",
						},
					},
				},
			},
		},
	}
}

func (d *TagKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *TagKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []tagKeyAPIResponse
	if err := d.client.Get(ctx, "/api/v1/workspaces/current/tag-keys", nil, &results); err != nil {
		resp.Diagnostics.AddError("Error listing tag keys", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for i := range results {
		tk := &results[i]
		desc := types.StringNull()
		if tk.Description != "" {
			desc = types.StringValue(tk.Description)
		}
		obj, diags := types.ObjectValue(tagKeyRecordObjectType.AttrTypes, map[string]attr.Value{
			"id":          types.StringValue(tk.ID),
			"key":         types.StringValue(tk.Key),
			"description": desc,
			"created_at":  types.StringValue(tk.CreatedAt),
			"updated_at":  types.StringValue(tk.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(tagKeyRecordObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.TagKeys = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
