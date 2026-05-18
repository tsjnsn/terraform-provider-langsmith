# List datasets in the current workspace with optional API filters.
# See the provider docs for supported query parameters (pagination, sort, name filters, etc.).

data "langsmith_datasets" "all" {}

output "dataset_ids" {
  value = [for d in data.langsmith_datasets.all.datasets : d.id]
}

data "langsmith_datasets" "recent_kv" {
  data_types = ["kv"]
  limit      = 20
  sort_by    = "modified_at"
}
