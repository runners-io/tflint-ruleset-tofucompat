resource "terraform_data" "marker" {}

moved {
  from = null_resource.marker
  to   = terraform_data.marker
}
