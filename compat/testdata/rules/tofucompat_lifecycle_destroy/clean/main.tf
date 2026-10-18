resource "terraform_data" "keep" {
  lifecycle {
    prevent_destroy = false
  }
}
