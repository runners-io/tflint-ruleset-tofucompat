variable "region" {
  type       = string
  default    = "eu-west-1"
  deprecated = "use var.location instead"
}

output "region" {
  value      = var.region
  deprecated = "read output.location instead"
}
