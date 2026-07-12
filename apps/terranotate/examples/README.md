# Terraform Directory Structure Examples

## Example 1: AWS Terraform Module with Sub-modules

This example demonstrates how to validate a Terraform module that contains sub-modules. It uses the AWS provider.

```
example1-aws-module/
├── schema.yaml
└── vpc/
    ├── main.tf                    # Main VPC configuration
    ├── variables.tf               # Module variables
    ├── outputs.tf                 # Module outputs
    ├── README.md
    └── modules/                   # Sub-modules
        ├── subnets/
        │   ├── main.tf
        │   ├── variables.tf
        │   └── outputs.tf
        ├── nat-gateway/
        │   ├── main.tf
        │   ├── variables.tf
        │   └── outputs.tf
        └── security-groups/
            ├── main.tf
            ├── variables.tf
            └── outputs.tf
```

**Validation Command:**
```bash
terranotate validate ./examples/example1-aws-module/vpc ./examples/example1-aws-module/schema.yaml
```

**What it validates:**
- All `.tf` files in `vpc/` (root module)
- All `.tf` files in `vpc/modules/*/` (sub-modules)
- Follows the standard Terraform module convention

## Example 2: Terraform Workspace

This example demonstrates recursively validating an entire infrastructure workspace with multiple environments and local modules.

```
example2-aws-workspace/
├── schema.yaml
└── infrastructure/
    ├── environments/
    │   ├── dev/
    │   │   ├── main.tf
    │   │   ├── terraform.tfvars
    │   │   └── backend.tf
    │   ├── staging/
    │   │   ├── main.tf
    │   │   ├── terraform.tfvars
    │   │   └── backend.tf
    │   └── production/
    │       ├── main.tf
    │       ├── terraform.tfvars
    │       └── backend.tf
    ├── modules/
    │   ├── app-server/
    │   │   ├── main.tf
    │   │   └── variables.tf
    │   └── database/
    │       ├── main.tf
    │       └── variables.tf
    ├── resources/
    │   ├── networking.tf
    │   ├── compute.tf
    │   ├── storage.tf
    │   └── security.tf
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    ├── terraform.tfvars
    ├── backend.tf
    └── providers.tf
```

**Validation Command:**
```bash
terranotate validate ./examples/example2-aws-workspace/infrastructure ./examples/example2-aws-workspace/schema.yaml
```

**What it validates:**
- All `.tf` files in root
- All `.tf` files in `environments/*/`
- All `.tf` files in `modules/*/`
- All `.tf` files in `resources/`
- Recursively scans all subdirectories

## Example 3: GCP Monorepo with Multiple Projects

This example demonstrates using the tool in a monorepo setup with GCP resources.

```
example3-gcp-monorepo/
├── schema.yaml
├── project-a/
│   ├── infrastructure/
│   │   ├── main.tf
│   │   ├── vpc.tf
│   │   └── modules/
│   │       └── app/
│   │           └── main.tf
│   └── README.md
├── project-b/
│   ├── infrastructure/
│   │   ├── main.tf
│   │   └── database.tf
│   └── README.md
└── shared-modules/
    ├── networking/
    │   └── main.tf
    └── security/
        └── main.tf
```

**Validation Commands:**
```bash
# Validate individual project
terranotate validate ./examples/example3-gcp-monorepo/project-a/infrastructure ./examples/example3-gcp-monorepo/schema.yaml

# Validate entire monorepo
terranotate validate ./examples/example3-gcp-monorepo ./examples/example3-gcp-monorepo/schema.yaml
```

## Example Output: Module Validation

```
=======================================================
Terranotate - Module Validation (with Sub-modules)
=======================================================

Module directory: ./examples/example1-aws-module/vpc
Schema file: ./examples/example1-aws-module/schema.yaml

Found 12 Terraform files across module and sub-modules:
  - main.tf
  - outputs.tf
  - variables.tf
  - modules/nat-gateway/main.tf
  - modules/nat-gateway/outputs.tf
  - modules/nat-gateway/variables.tf
  - modules/security-groups/main.tf
  - modules/security-groups/outputs.tf
  - modules/security-groups/variables.tf
  - modules/subnets/main.tf
  - modules/subnets/outputs.tf
  - modules/subnets/variables.tf

================================================================================
MODULE VALIDATION RESULTS
================================================================================

✅ Module validation passed!
   All files in ./examples/example1-aws-module/vpc meet schema requirements
```

## Example Output: Workspace Validation (with errors)

```
=========================================================
Terranotate - Workspace Validation (Recursive)
=========================================================

Workspace directory: ./examples/example2-aws-workspace/infrastructure
Schema file: ./examples/example2-aws-workspace/schema.yaml

Found 19 Terraform files in 7 directories:

  📁 root (5 files)
    - backend.tf
    - main.tf
    - outputs.tf
    - providers.tf
    - variables.tf

  📁 environments/dev (2 files)
  📁 environments/production (2 files)
  📁 environments/staging (2 files)
  📁 modules/app-server (2 files)
  📁 modules/database (2 files)
  📁 resources (4 files)

================================================================================
WORKSPACE VALIDATION RESULTS
================================================================================

❌ Workspace validation failed for: ./examples/example2-aws-workspace/infrastructure

================================================================================

📁 Directory: environments/production (2 errors)
--------------------------------------------------------------------------------
  ❌ [ERROR] aws_instance (main.tf) - Line 3
     Missing required comment prefix: @validation

  ❌ [ERROR] aws_rds_cluster (main.tf) - Line 8
     @validation: Missing required field 'backup_required'


📁 Directory: modules/app-server (1 errors)
--------------------------------------------------------------------------------
  ❌ [ERROR] aws_security_group (main.tf) - Line 1
     @metadata: Missing nested structure 'contact'


📁 Directory: modules/database (1 errors)
--------------------------------------------------------------------------------
  ❌ [ERROR] aws_db_instance (main.tf) - Line 1
     @metadata: Missing nested structure 'contact'


📁 Directory: resources (4 errors)
--------------------------------------------------------------------------------
  ❌ [ERROR] aws_instance (compute.tf) - Line 3
     Missing required comment prefix: @validation

  ❌ [ERROR] aws_vpc (networking.tf) - Line 1
     @metadata: Missing nested structure 'contact'

  ❌ [ERROR] aws_iam_role (security.tf) - Line 1
     @metadata: Missing nested structure 'contact'

  ❌ [ERROR] aws_s3_bucket (storage.tf) - Line 1
     @metadata: Missing nested structure 'contact'

================================================================================

Total errors: 8 across 4 directories
```

## Example 4: GCP Module with Fix Command

This example demonstrates how to use the `fix` command to automatically add missing annotations to your Terraform files.

```
example4-gcp-module/
├── schema.yaml
└── storage/
    └── main.tf                    # Storage bucket without annotations
```

**Validation Command (Expected to FAIL):**
```bash
terranotate validate ./examples/example4-gcp-module/storage ./examples/example4-gcp-module/schema.yaml
```

**Fix Command (Expected to PASS):**
```bash
terranotate fix ./examples/example4-gcp-module/storage ./examples/example4-gcp-module/schema.yaml
```

**What it does:**
- `validate` will find a `google_storage_bucket` without any `@metadata` or `@validation` tags.
- `fix` will automatically add the missing comment blocks with placeholder values (e.g., `# @metadata owner:CHANGEME`, `# @validation priority:medium`).


## Skipped Directories

The validator automatically skips:
- `.terraform/` - Terraform plugin cache
- `terraform.tfstate.d/` - Terraform workspace states
- `.git/` - Git repository
- `node_modules/` - Node.js dependencies
- Any directory starting with `.` (hidden directories)
