locals {
  greeting = "hello $${name}"
}

output "greeting" {
  value = templatestring(local.greeting, { name = "world" })
}
