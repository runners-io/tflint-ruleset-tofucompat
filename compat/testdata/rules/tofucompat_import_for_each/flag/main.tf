locals {
  buckets = { logs = "logs", data = "data" }
}

resource "terraform_data" "bucket" {
  for_each = local.buckets
}

import {
  for_each = local.buckets
  to       = terraform_data.bucket[each.key]
  id       = each.value
}
