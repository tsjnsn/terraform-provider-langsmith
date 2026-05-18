// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &DatasetsDataSource{}

// datasetListItemAttrTypes is the object shape of each element in `datasets`
// (aligned with the `langsmith_dataset` data source attributes).
var datasetListItemAttrTypes = map[string]attr.Type{
	"id":                        types.StringType,
	"name":                      types.StringType,
	"description":               types.StringType,
	"data_type":                 types.StringType,
	"inputs_schema_definition":  types.StringType,
	"outputs_schema_definition": types.StringType,
	"externally_managed":        types.BoolType,
	"transformations":           types.StringType,
	"metadata":                  types.StringType,
	"tenant_id":                 types.StringType,
	"created_at":                types.StringType,
	"modified_at":               types.StringType,
	"example_count":             types.Int64Type,
	"session_count":             types.Int64Type,
	"last_session_start_time":   types.StringType,
}

// NewDatasetsDataSource returns a data source backed by GET /api/v1/datasets.
func NewDatasetsDataSource() datasource.DataSource {
	return &DatasetsDataSource{}
}

// DatasetsDataSource lists datasets in the configured workspace (tenant).
type DatasetsDataSource struct {
	client *client.Client
}

// DatasetsDataSourceModel is Terraform state for the datasets listing.
type DatasetsDataSourceModel struct {
	ID types.String `tfsdk:"id"`

	Ids                        types.List   `tfsdk:"ids"`
	DataTypes                  types.List   `tfsdk:"data_types"`
	Name                       types.String `tfsdk:"name"`
	NameContains               types.String `tfsdk:"name_contains"`
	Metadata                   types.String `tfsdk:"metadata"`
	Offset                     types.Int64  `tfsdk:"offset"`
	Limit                      types.Int64  `tfsdk:"limit"`
	SortBy                     types.String `tfsdk:"sort_by"`
	SortByDesc                 types.Bool   `tfsdk:"sort_by_desc"`
	TagValueIds                types.List   `tfsdk:"tag_value_ids"`
	ExcludeCorrectionsDatasets types.Bool   `tfsdk:"exclude_corrections_datasets"`
	Exclude                    types.List   `tfsdk:"exclude"`

	Datasets types.List `tfsdk:"datasets"`
}

func (d *DatasetsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasets"
}

func (d *DatasetsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	sortByMarkdown := "Sort column from OpenAPI `SortByDatasetColumn`: `name`, `created_at`, `last_session_start_time`, `example_count`, `session_count`, or `modified_at`. When unset, the API default (`last_session_start_time`) applies."

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith **datasets** in the current workspace via GET [`/api/v1/datasets`](https://api.smith.langchain.com/openapi.json). " +
			"Optional arguments mirror the API's query parameters for filtering, pagination, and sorting. " +
			"Repeated query keys are mapped from Terraform lists: `ids` → `id`, `data_types` → `data_type`, `tag_value_ids` → `tag_value_id`, and `exclude` → `exclude` (OpenAPI currently defines only the `example_count` value for `exclude`). " +
			"The API caps `limit` at 100 (OpenAPI default 100). " +
			"The `metadata` argument is forwarded as a single string; consult the LangSmith API for the expected encoding. " +
			"Nested `datasets` attributes match the `langsmith_dataset` data source (the OpenAPI `Dataset.baseline_experiment_id` field is not included there and is omitted here as well). " +
			"For a single dataset by id or name, use the `langsmith_dataset` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable placeholder (`datasets`) so the data source can be referenced.",
				Computed:            true,
			},
			"ids": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				MarkdownDescription: "Dataset UUIDs to filter by; sent as repeated `id` query parameters. " +
					"The LangSmith OpenAPI types this as an array of UUID strings.",
			},
			"data_types": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				MarkdownDescription: "Filter by one or more dataset `data_type` values; sent as repeated `data_type` query keys. " +
					"Each element must be `kv`, `llm`, or `chat` per OpenAPI `DataType`.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("kv", "llm", "chat")),
				},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Exact name filter (`name` query parameter).",
			},
			"name_contains": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Substring name filter (`name_contains` query parameter).",
			},
			"metadata": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Metadata filter string passed through to the API (`metadata` query parameter). Format is defined by the LangSmith API.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Pagination offset (`offset`, default 0 in OpenAPI).",
				Validators:          []validator.Int64{int64validator.AtLeast(0)},
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Page size (`limit`). OpenAPI allows 1–100 (default 100).",
				Validators:          []validator.Int64{int64validator.Between(1, 100)},
			},
			"sort_by": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: sortByMarkdown,
				Validators: []validator.String{
					stringvalidator.OneOf("name", "created_at", "last_session_start_time", "example_count", "session_count", "modified_at"),
				},
			},
			"sort_by_desc": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When `true`, descending sort; when `false`, ascending. When unset, the API default (`true`) applies.",
			},
			"tag_value_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Tag value UUIDs to filter by; sent as repeated `tag_value_id` query parameters.",
			},
			"exclude_corrections_datasets": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When `true`, sends `exclude_corrections_datasets=true`. When unset, the API default (`false`) applies.",
			},
			"exclude": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Sent as repeated `exclude` query parameters. OpenAPI only defines `example_count` today (`GetDatasetsSelect`).",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("example_count")),
				},
			},
			"datasets": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Dataset UUID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Dataset name.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Dataset description.",
						},
						"data_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Dataset data type (`kv`, `llm`, or `chat`).",
						},
						"inputs_schema_definition": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "JSON string of the inputs JSON schema definition.",
						},
						"outputs_schema_definition": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "JSON string of the outputs JSON schema definition.",
						},
						"externally_managed": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the dataset is externally managed.",
						},
						"transformations": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "JSON string of the dataset transformations.",
						},
						"metadata": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "JSON string of the dataset metadata.",
						},
						"tenant_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Workspace (tenant) UUID.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Creation timestamp.",
						},
						"modified_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Last modification timestamp.",
						},
						"example_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Number of examples in the dataset.",
						},
						"session_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Number of sessions associated with the dataset.",
						},
						"last_session_start_time": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Start time of the last session, if any.",
						},
					},
				},
				MarkdownDescription: "Datasets returned by the API for the current request (up to `limit`).",
			},
		},
	}
}

func (d *DatasetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *DatasetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatasetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query, qDiags := datasetsListQueryValues(ctx, &data)
	resp.Diagnostics.Append(qDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []datasetDataSourceAPIResponse
	if err := d.client.Get(ctx, "/api/v1/datasets", query, &results); err != nil {
		resp.Diagnostics.AddError("Error reading datasets", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for i := range results {
		obj := types.ObjectValueMust(datasetListItemAttrTypes, datasetListItemObjectMap(&results[i]))
		elems = append(elems, obj)
	}

	listVal, listDiags := types.ListValue(types.ObjectType{AttrTypes: datasetListItemAttrTypes}, elems)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("datasets")
	data.Datasets = listVal

	tflog.Trace(ctx, "read datasets data source", map[string]any{"count": len(results)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func datasetsListQueryValues(ctx context.Context, data *DatasetsDataSourceModel) (url.Values, diag.Diagnostics) {
	var diags diag.Diagnostics
	q := url.Values{}

	var ids []string
	if !data.Ids.IsNull() && !data.Ids.IsUnknown() {
		diags.Append(data.Ids.ElementsAs(ctx, &ids, false)...)
		for _, id := range ids {
			q.Add("id", id)
		}
	}

	var dataTypes []string
	if !data.DataTypes.IsNull() && !data.DataTypes.IsUnknown() {
		diags.Append(data.DataTypes.ElementsAs(ctx, &dataTypes, false)...)
		for _, dt := range dataTypes {
			q.Add("data_type", dt)
		}
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		q.Set("name", data.Name.ValueString())
	}
	if !data.NameContains.IsNull() && !data.NameContains.IsUnknown() {
		q.Set("name_contains", data.NameContains.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		q.Set("metadata", data.Metadata.ValueString())
	}

	if !data.Offset.IsNull() && !data.Offset.IsUnknown() {
		q.Set("offset", strconv.FormatInt(data.Offset.ValueInt64(), 10))
	}
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		q.Set("limit", strconv.FormatInt(data.Limit.ValueInt64(), 10))
	}

	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		q.Set("sort_by", data.SortBy.ValueString())
	}

	if !data.SortByDesc.IsNull() && !data.SortByDesc.IsUnknown() {
		if data.SortByDesc.ValueBool() {
			q.Set("sort_by_desc", "true")
		} else {
			q.Set("sort_by_desc", "false")
		}
	}

	var tagIDs []string
	if !data.TagValueIds.IsNull() && !data.TagValueIds.IsUnknown() {
		diags.Append(data.TagValueIds.ElementsAs(ctx, &tagIDs, false)...)
		for _, tid := range tagIDs {
			q.Add("tag_value_id", tid)
		}
	}

	if !data.ExcludeCorrectionsDatasets.IsNull() && !data.ExcludeCorrectionsDatasets.IsUnknown() && data.ExcludeCorrectionsDatasets.ValueBool() {
		q.Set("exclude_corrections_datasets", "true")
	}

	var exclude []string
	if !data.Exclude.IsNull() && !data.Exclude.IsUnknown() {
		diags.Append(data.Exclude.ElementsAs(ctx, &exclude, false)...)
		for _, ex := range exclude {
			q.Add("exclude", ex)
		}
	}

	return q, diags
}

func datasetListItemObjectMap(result *datasetDataSourceAPIResponse) map[string]attr.Value {
	m := map[string]attr.Value{
		"id":          types.StringValue(result.ID),
		"name":        types.StringValue(result.Name),
		"data_type":   types.StringValue(result.DataType),
		"tenant_id":   types.StringValue(result.TenantID),
		"created_at":  types.StringValue(result.CreatedAt),
		"modified_at": types.StringValue(result.ModifiedAt),
	}

	if result.ExampleCount != nil {
		m["example_count"] = types.Int64Value(*result.ExampleCount)
	} else {
		m["example_count"] = types.Int64Null()
	}

	if result.Description != nil {
		m["description"] = types.StringValue(*result.Description)
	} else {
		m["description"] = types.StringNull()
	}

	if len(result.InputsSchemaDefinition) > 0 && string(result.InputsSchemaDefinition) != "null" {
		m["inputs_schema_definition"] = types.StringValue(string(result.InputsSchemaDefinition))
	} else {
		m["inputs_schema_definition"] = types.StringNull()
	}

	if len(result.OutputsSchemaDefinition) > 0 && string(result.OutputsSchemaDefinition) != "null" {
		m["outputs_schema_definition"] = types.StringValue(string(result.OutputsSchemaDefinition))
	} else {
		m["outputs_schema_definition"] = types.StringNull()
	}

	if result.ExternallyManaged != nil {
		m["externally_managed"] = types.BoolValue(*result.ExternallyManaged)
	} else {
		m["externally_managed"] = types.BoolNull()
	}

	if len(result.Transformations) > 0 && string(result.Transformations) != "null" {
		m["transformations"] = types.StringValue(string(result.Transformations))
	} else {
		m["transformations"] = types.StringNull()
	}

	if len(result.Metadata) > 0 && string(result.Metadata) != "null" {
		m["metadata"] = types.StringValue(string(result.Metadata))
	} else {
		m["metadata"] = types.StringNull()
	}

	if result.SessionCount != nil {
		m["session_count"] = types.Int64Value(*result.SessionCount)
	} else {
		m["session_count"] = types.Int64Null()
	}

	if result.LastSessionStartTime != nil {
		m["last_session_start_time"] = types.StringValue(*result.LastSessionStartTime)
	} else {
		m["last_session_start_time"] = types.StringNull()
	}

	return m
}
