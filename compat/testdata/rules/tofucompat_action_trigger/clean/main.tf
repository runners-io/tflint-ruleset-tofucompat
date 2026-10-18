resource "terraform_data" "deploy" {
  input = "v1"

  lifecycle {
    create_before_destroy = true
  }
}
