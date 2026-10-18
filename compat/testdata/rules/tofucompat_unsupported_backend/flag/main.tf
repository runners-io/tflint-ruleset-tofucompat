terraform {
  backend "oci" {
    bucket    = "tfstate"
    namespace = "acme"
  }
}
