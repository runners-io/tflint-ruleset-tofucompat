variable "region" {
  type       = string
  default    = "eu-west-1"
}

output "region" {
  value      = var.region
}
