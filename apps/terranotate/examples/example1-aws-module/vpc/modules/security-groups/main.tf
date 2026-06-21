# @metadata owner:security-team
resource "aws_security_group" "allow_web" {
  name        = "allow_web_traffic"
  description = "Allow inbound web traffic"
  vpc_id      = var.vpc_id
}
