# Advanced Usage & Customization

## Create Custom Schemas

### Step 1: Define Resource Types

Identify which resource types need validation:

```yaml
resource_types:
  aws_instance:
    # ... rules ...
  
  aws_rds_cluster:
    # ... rules ...
```

### Step 2: Define Required Prefixes

Specify which comment types are mandatory:

```yaml
required_prefixes:
  - "@metadata"
  - "@validation"
```

### Step 3: Define Field Requirements

Specify required and optional fields:

```yaml
prefix_rules:
  "@metadata":
    required_fields:
      - owner
      - team
    optional_fields:
      - priority
      - cost_center
```

### Step 4: Define Nested Structures

For complex nested fields:

```yaml
nested_fields:
  contact:
    required_fields:
      - email
    optional_fields:
      - slack
      - phone
  
  sla:
    required_fields:
      - uptime
```

### Step 5: Add Value Validations

Define constraints on field values:

```yaml
field_validations:
  environment:
    type: string
    allowed_values:
      - development
      - staging
      - production
  
  port:
    type: integer
    min: 1
    max: 65535
  
  uptime:
    type: float
    min: 0.0
    max: 100.0
```

## Add More Prefixes

In your code, if you extend the tool:

```go
prefixes := []string{
    "@metadata",
    "@docs", 
    "@validation",
    "@config",
    "@security",    // Add custom prefixes
    "@compliance",
    "@monitoring",
}
```

## Documentation Generation

The `generate` command allows you to create Markdown documentation directly from your Terraform resources and their annotations.

### Basic Generation

```bash
terranotate generate path/to/terraform schema.yaml
```

### Advanced Options

- **Output to File**: Use the `--output` flag to save the documentation to a file.
- **Custom Schema**: Provide a specific schema to focus the documentation on certain fields.

### Customize Documentation

The generator uses the `required_fields` defined in your schema to determine which columns to show in the output tables. If the schema defines no fields for a resource type, the generator shows the resource name and description by default.

```bash
# Example: Generate a compliance report
terranotate generate ./production production-schema.yaml --output compliance-report.md
```
