terraform {
  required_providers {
    corefunc = {
      source  = "northwood-labs/corefunc"
      version = "1.4.0"
    }
  }
}

output "name" {
  value = provider::corefunc::str_camel("hello world")
}
