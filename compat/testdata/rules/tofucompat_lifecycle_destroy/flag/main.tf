resource "terraform_data" "keep" {
  lifecycle {
    destroy = false
  }
}
