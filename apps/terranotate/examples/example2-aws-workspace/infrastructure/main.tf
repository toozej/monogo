provider "aws" {
  region = var.region
}

module "app" {
  source = "./modules/app-server"
}
