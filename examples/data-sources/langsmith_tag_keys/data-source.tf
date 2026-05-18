data "langsmith_tag_keys" "all" {}

output "tag_key_names" {
  value = [for tk in data.langsmith_tag_keys.all.tag_keys : tk.key]
}
