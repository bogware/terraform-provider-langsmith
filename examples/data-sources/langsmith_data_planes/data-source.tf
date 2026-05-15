data "langsmith_data_planes" "all" {}

output "data_plane_names" {
  value = [for dp in data.langsmith_data_planes.all.data_planes : dp.name]
}
