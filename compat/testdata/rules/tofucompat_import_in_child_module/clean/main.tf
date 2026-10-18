module "network" {
  source = "./network"
}

import {
  to = module.network.terraform_data.vpc
  id = "vpc-123"
}
