variable "password" {
  type      = string
  default   = "dev"
  sensitive = true
}

output "password_is_sensitive" {
  value = issensitive(var.password)
}
