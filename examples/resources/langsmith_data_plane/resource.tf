# Provisions a BYOC data plane for the current organization (org-scoped;
# requires BYOC enabled and org-admin permissions). The API has no update or
# delete endpoint: changes force replacement, and destroy only removes the
# resource from state.
resource "langsmith_data_plane" "us_east" {
  name        = "prod-us-east-1"
  region      = "us-east-1"
  external_id = var.langsmith_external_id
  role_arn    = "arn:aws:iam::123456789012:role/langsmith-byoc"
  vpc_cidr    = "10.42.0.0/16"
}
