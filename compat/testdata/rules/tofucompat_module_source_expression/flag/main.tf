locals {
  network_module = "./network"
}

module "network" {
  source = local.network_module
}
