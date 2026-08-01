data "langsmith_info_health" "example" {}

output "clickhouse_disk_free_pct" {
  value = data.langsmith_info_health.example.clickhouse_disk_free_pct
}
