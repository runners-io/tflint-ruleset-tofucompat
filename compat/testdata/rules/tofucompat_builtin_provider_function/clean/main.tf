terraform {
  required_providers {
    terraform = {
      source = "terraform.io/builtin/terraform"
    }
  }
}

output "tfvars" {
  value = "region = \"eu-west-1\""
}
