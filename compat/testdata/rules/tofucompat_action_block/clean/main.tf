resource "terraform_data" "deploy" {
  input = "v1"
}

resource "terraform_data" "ping" {
  triggers_replace = [terraform_data.deploy.output]
}
