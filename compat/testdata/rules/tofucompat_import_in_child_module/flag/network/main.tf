resource "terraform_data" "vpc" {}

import {
  to = terraform_data.vpc
  id = "vpc-123"
}
