# @metadata owner:sre-team
# This resource is missing @validation prefix
resource "aws_instance" "prod_web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "m5.large"
}

# @validation
# This resource is missing required field 'backup_required'
resource "aws_rds_cluster" "prod_db" {
  cluster_identifier = "prod-db"
  engine             = "aurora-postgresql"
}
