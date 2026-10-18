resource "terraform_data" "marker" {}

moved {
  from = terraform_data.old_marker
  to   = terraform_data.marker
}
