# Look up a platform tool by stable handle
data "langsmith_tool" "by_handle" {
  handle = "example-tool-handle"
}

# Or by UUID
# data "langsmith_tool" "by_id" {
#   id = "00000000-0000-0000-0000-000000000000"
# }

output "tool_name" {
  value = data.langsmith_tool.by_handle.name
}
