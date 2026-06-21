# @metadata owner:alice.smith team:infrastructure department:platform
# priority:critical cost_center:engineering
# contact.email:alice@example.com contact.slack:@alice contact.phone:555-0123
# tags:[production,critical,us-east-1,terraform-managed]
# @docs description:Primary web application server for production environment
# This server handles all incoming HTTP/HTTPS traffic
# @validation required:true environment:production
# compliance.soc2:true compliance.hipaa:true compliance.pci:false
# security.encrypted:true security.backup:true
resource "aws_instance" "web_server" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t3.medium"
  
  # @config autoscaling:true min_instances:2 max_instances:10
  # scaling.cpu_threshold:75.5 scaling.memory_threshold:80.0
  # monitoring.enabled:true monitoring.interval:60
  
  tags = {
    Name        = "WebServer"
    Environment = "production"
  }
}

# @metadata owner:bob.jones team:data-engineering
# contact.email:bob@example.com
# tags:[development,non-critical]
# @config backup.enabled:true backup.retention_days:30
# backup.schedule:daily backup.time:02:00
resource "aws_s3_bucket" "data_lake" {
  bucket = "company-data-lake-prod"
  
  # @validation encryption:required versioning:required
  # compliance.gdpr:true
}

# Multi-line documentation comment
# @docs description:Database cluster for customer data storage
# This is a critical component that requires 99.99% uptime
# All changes must be reviewed by the DBA team
# Connection pooling is enabled with max_connections set to 100
# @metadata owner:carol.white team:database-admin
# priority:critical emergency_contact:dba-oncall@example.com
# contact.primary.name:Carol_White contact.primary.email:carol@example.com
# contact.secondary.name:Dave_Brown contact.secondary.email:dave@example.com
# sla.uptime:99.99 sla.response_time:500
# @config ha.enabled:true ha.replicas:3 ha.zone_distribution:[us-east-1a,us-east-1b,us-east-1c]
# performance.iops:10000 performance.storage_type:io2
# @validation backup_required:true point_in_time_recovery:true
# security.encryption_at_rest:true security.encryption_in_transit:true
resource "aws_rds_cluster" "customer_db" {
  cluster_identifier = "customer-data-prod"
  engine            = "aurora-postgresql"
  engine_version    = "13.7"
  
  # @config maintenance.window:sun:03:00-sun:04:00
  # maintenance.auto_minor_version_upgrade:false
}

# @metadata owner:eve.davis team:networking
# tags:[infrastructure,core]
# @config cidr.blocks:[10.0.0.0/16,10.1.0.0/16]
# subnets.public:3 subnets.private:3
# nat.enabled:true nat.high_availability:true
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
  
  # @validation dns_support:required dns_hostnames:required
}

# Simple inline comment example
# @metadata owner:frank.miller team:security priority:high
# @validation mfa_required:true password_policy:strict
resource "aws_iam_role" "app_role" {
  name = "application-role"
  # @config assume_role.enabled:true assume_role.duration:3600
}

# @metadata owner:grace.lee team:devops
# cost_center:operations budget.monthly:5000 budget.currency:USD
# tags:[automation,ci-cd,jenkins]
# notification.email:devops@example.com notification.slack:#devops-alerts
# @config pipeline.stages:[build,test,deploy]
# pipeline.parallel_builds:5 pipeline.timeout:1800
# build.docker:true build.cache:true
# @docs description:CI/CD pipeline infrastructure for automated deployments
# Supports multiple concurrent builds with Docker caching enabled
resource "aws_codepipeline" "main" {
  name     = "main-pipeline"
  role_arn = "arn:aws:iam::123456789012:role/pipeline-role"
}

# @metadata owner:henry.wilson team:monitoring priority:medium
# oncall.primary:monitoring-team@example.com
# oncall.escalation.level1:team-lead@example.com
# oncall.escalation.level2:director@example.com
# integration.pagerduty:true integration.slack:true
# @config alerts.cpu:true alerts.memory:true alerts.disk:true
# thresholds.cpu.warning:70 thresholds.cpu.critical:90
# thresholds.memory.warning:75.5 thresholds.memory.critical:95.0
# retention.metrics:90 retention.logs:30
# @validation alerting_required:true dashboards_required:true
# @docs description:CloudWatch monitoring and alerting configuration
# Monitors key metrics and sends alerts to PagerDuty and Slack
# Retention policies comply with company data governance standards
resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "main-dashboard"
}

# Boolean and numeric type examples
# @metadata active:true deprecated:false
# version:2 revision:3.14
# @config feature_flags.new_ui:true feature_flags.beta_api:false
# limits.max_requests:1000 limits.rate_limit:100.5
resource "aws_api_gateway_rest_api" "api" {
  name = "main-api"
}

# Array and nested structure examples
# @metadata environments:[dev,staging,prod]
# regions:[us-east-1,us-west-2,eu-west-1]
# @config cache.layers:[L1,L2,L3]
# cache.ttl.L1:300 cache.ttl.L2:3600 cache.ttl.L3:86400
# access.allowed_ips:[10.0.0.0/8,172.16.0.0/12]
# access.blocked_countries:[XX,YY,ZZ]
resource "aws_cloudfront_distribution" "cdn" {
  enabled = true
}
