variable "password" {
  type      = string
  default   = "dev"
  sensitive = true
}

output "password_length" {
  value     = length(var.password)
  sensitive = true
}
