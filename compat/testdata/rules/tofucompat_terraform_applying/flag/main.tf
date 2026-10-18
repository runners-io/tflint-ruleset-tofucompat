locals {
  mode = terraform.applying ? "apply" : "plan"
}
