# @metadata owner:app-team
# Missing contact.email nested field
resource "aws_security_group" "app_sg" {
  name        = "app-sg"
  description = "Security group for app server"
}
