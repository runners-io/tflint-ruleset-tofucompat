output "tfvars" {
  value = provider::terraform::encode_tfvars({ region = "eu-west-1" })
}
