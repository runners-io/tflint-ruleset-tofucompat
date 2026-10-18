locals {
  buckets = { logs = "logs", data = "data" }
}

resource "terraform_data" "bucket" {
  for_each = local.buckets
}
