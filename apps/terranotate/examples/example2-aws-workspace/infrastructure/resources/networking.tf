# @metadata owner:networking-team
resource "aws_vpc" "secondary" {
  cidr_block = "10.1.0.0/16"
}
