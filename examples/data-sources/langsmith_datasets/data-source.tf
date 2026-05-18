# List the first page of datasets in the current workspace.
# See the provider docs for pagination, sorting, and additional filters.

data "langsmith_datasets" "first_page" {}

output "dataset_ids" {
  value = [for d in data.langsmith_datasets.first_page.datasets : d.id]
}

data "langsmith_datasets" "recent_kv" {
  data_types = ["kv"]
  limit      = 20
  sort_by    = "modified_at"
}
