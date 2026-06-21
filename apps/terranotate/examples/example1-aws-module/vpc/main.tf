provider "aws" {
  region = "us-east-1"
}

# @metadata owner:networking-team team:infrastructure
resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr
  
  tags = {
    Name = "main-vpc"
  }
}

module "subnets" {
  source = "./modules/subnets"
  vpc_id = aws_vpc.main.id
}
