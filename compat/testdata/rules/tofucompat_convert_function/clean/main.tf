variable "port" {
  type    = string
  default = "8080"
}

output "port_number" {
  value = tonumber(var.port)
}
