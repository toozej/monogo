# API Usage Examples

## Extract Specific Resource

```go
for _, resource := range resources {
    if resource.Type == "aws_instance" && resource.Name == "web_server" {
        // Work with this specific resource
    }
}
```

## Get All Metadata

```go
metadata := resource.GetCommentsByPrefix("@metadata")
for _, m := range metadata {
    owner := m.Fields["owner"]
    team := m.Fields["team"]
}
```

## Access Nested Fields

```go
// Using helper method
email := resource.GetNestedField("@metadata", "contact.email")

// Manual navigation
metadata := resource.GetCommentsByPrefix("@metadata")
if len(metadata) > 0 {
    if contact, ok := metadata[0].Fields["contact"].(map[string]interface{}); ok {
        email := contact["email"]
    }
}
```

## Type Assertions

```go
// Boolean
if enabled, ok := config.Fields["enabled"].(bool); ok {
    fmt.Printf("Enabled: %t\n", enabled)
}

// Integer
if port, ok := config.Fields["port"].(int); ok {
    fmt.Printf("Port: %d\n", port)
}

// Float
if threshold, ok := config.Fields["threshold"].(float64); ok {
    fmt.Printf("Threshold: %.2f\n", threshold)
}

// Array
if tags, ok := config.Fields["tags"].([]interface{}); ok {
    for _, tag := range tags {
        fmt.Printf("Tag: %s\n", tag)
    }
}

// Nested map
if contact, ok := config.Fields["contact"].(map[string]interface{}); ok {
    email := contact["email"]
}
```
