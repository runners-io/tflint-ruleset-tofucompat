terraform {
  cloud {
    organization = "acme"

    workspaces {
      name = "app"
    }
  }
}
