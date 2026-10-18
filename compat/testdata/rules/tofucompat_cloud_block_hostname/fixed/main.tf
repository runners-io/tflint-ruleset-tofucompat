terraform {
  cloud {
    hostname = "app.terraform.io"
    organization = "acme"

    workspaces {
      name = "app"
    }
  }
}
