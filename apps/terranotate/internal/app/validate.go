package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
)

// Validate implements the validate command logic
func Validate(fs afero.Fs, terraformFile, schemaFile string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Schema Validation")
	fmt.Println("=================================================")
	fmt.Printf("Terraform file: %s\n", terraformFile)
	fmt.Printf("Schema file: %s\n\n", schemaFile)

	// Parse the Terraform file
	prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
	p := parser.NewCommentParser(fs, prefixes)

	resources, err := p.ParseFile(terraformFile)
	if err != nil {
		return fmt.Errorf("failed to parse Terraform file: %w", err)
	}

	fmt.Printf("Parsed %d resources\n", len(resources))

	// Load and validate against schema
	v, err := validator.NewSchemaValidator(fs, schemaFile)
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	fmt.Println("Validating against schema...")

	result := v.ValidateResources(resources)

	validator.PrintValidationResults(result)

	if !result.Passed {
		return fmt.Errorf("\n💡 Tip: Run 'terranotate fix %s %s' to auto-fix some issues", terraformFile, schemaFile)
	}

	return nil
}

// ValidateAuto automatically detects the type of path and validates accordingly
func ValidateAuto(fs afero.Fs, path, schemaFile string) error {
	// Check if path exists
	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	// If it's a single file, validate as single file
	if !info.IsDir() {
		return Validate(fs, path, schemaFile)
	}

	// It's a directory - detect whether it's a module or workspace
	detectedType := detectDirectoryType(fs, path)

	switch detectedType {
	case "workspace":
		fmt.Println("🔍 Auto-detected: Terraform Workspace")
		return ValidateWorkspace(fs, path, schemaFile)
	case "module":
		fmt.Println("🔍 Auto-detected: Terraform Module")
		return ValidateModule(fs, path, schemaFile)
	default:
		// Default to single directory validation (treat as simple terraform directory)
		fmt.Println("🔍 Auto-detected: Terraform Directory")
		return validateDirectory(fs, path, schemaFile)
	}
}

// detectDirectoryType determines if a directory is a module, workspace, or simple directory
func detectDirectoryType(fs afero.Fs, path string) string {
	// Check for modules/ subdirectory (indicates this is likely a module)
	modulesPath := filepath.Join(path, "modules")
	if info, err := fs.Stat(modulesPath); err == nil && info.IsDir() {
		return "module"
	}

	// Check if path itself is inside a modules/ directory
	if strings.Contains(path, string(filepath.Separator)+"modules"+string(filepath.Separator)) {
		return "module"
	}

	// Check for typical workspace indicators
	// - environments/ directory
	// - Multiple terraform state configurations
	// - infrastructure/ directory (common workspace pattern)
	workspaceIndicators := []string{"environments", "infrastructure", "env"}
	for _, indicator := range workspaceIndicators {
		indicatorPath := filepath.Join(path, indicator)
		if info, err := fs.Stat(indicatorPath); err == nil && info.IsDir() {
			// Check if this directory has subdirectories (environments)
			if hasSubdirectories(fs, indicatorPath) {
				return "workspace"
			}
		}
	}

	// If we find multiple terraform directories at the top level, treat as workspace
	terraformDirs := 0
	entries, err := afero.ReadDir(fs, path)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				entryPath := filepath.Join(path, entry.Name())
				if hasTerraformFiles(fs, entryPath) {
					terraformDirs++
				}
			}
		}
		if terraformDirs > 2 {
			return "workspace"
		}
	}

	// Default to simple directory
	return "directory"
}

// hasSubdirectories checks if a directory contains subdirectories
func hasSubdirectories(fs afero.Fs, path string) bool {
	entries, err := afero.ReadDir(fs, path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			return true
		}
	}
	return false
}

// hasTerraformFiles checks if a directory contains any .tf files
func hasTerraformFiles(fs afero.Fs, path string) bool {
	entries, err := afero.ReadDir(fs, path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tf") {
			return true
		}
	}
	return false
}

// validateDirectory validates all .tf files in a single directory (non-recursive)
func validateDirectory(fs afero.Fs, dir, schemaFile string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Directory Validation")
	fmt.Println("=================================================")
	fmt.Printf("Directory: %s\n", dir)
	fmt.Printf("Schema file: %s\n\n", schemaFile)

	// Find .tf files in the directory (non-recursive)
	entries, err := afero.ReadDir(fs, dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var tfFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tf") {
			tfFiles = append(tfFiles, filepath.Join(dir, entry.Name()))
		}
	}

	if len(tfFiles) == 0 {
		return fmt.Errorf("no Terraform files found in directory: %s", dir)
	}

	fmt.Printf("Found %d Terraform file(s):\n", len(tfFiles))
	for _, file := range tfFiles {
		fmt.Printf("  - %s\n", filepath.Base(file))
	}
	fmt.Println()

	// Parse and validate all files
	prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
	p := parser.NewCommentParser(fs, prefixes)

	var allResources []parser.TerraformResource
	for _, file := range tfFiles {
		resources, err := p.ParseFile(file)
		if err != nil {
			log.Printf("Warning: Failed to parse %s: %v", file, err)
			continue
		}
		allResources = append(allResources, resources...)
	}

	fmt.Printf("Parsed %d total resources\n", len(allResources))

	// Load and validate against schema
	v, err := validator.NewSchemaValidator(fs, schemaFile)
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	fmt.Println("Validating against schema...")

	result := v.ValidateResources(allResources)

	validator.PrintValidationResults(result)

	if !result.Passed {
		return fmt.Errorf("\n💡 Tip: Run 'terranotate fix %s %s' to auto-fix some issues", dir, schemaFile)
	}

	return nil
}

// ValidateModule implements the validate-module command logic
func ValidateModule(fs afero.Fs, moduleDir, schemaFile string) error {
	fmt.Println("=======================================================")
	fmt.Println("Terranotate - Module Validation (with Sub-modules)")
	fmt.Println("=======================================================")
	fmt.Printf("Module directory: %s\n", moduleDir)
	fmt.Printf("Schema file: %s\n\n", schemaFile)

	// Validate the module structure
	if err := validateModuleStructure(fs, moduleDir); err != nil {
		return fmt.Errorf("invalid module structure: %w", err)
	}

	// Find all Terraform files in the module and sub-modules
	tfFiles, err := findModuleTerraformFiles(fs, moduleDir)
	if err != nil {
		return fmt.Errorf("failed to scan module directory: %w", err)
	}

	if len(tfFiles) == 0 {
		return fmt.Errorf("no Terraform files found in module: %s", moduleDir)
	}

	fmt.Printf("Found %d Terraform files across module and sub-modules:\n", len(tfFiles))
	for _, file := range tfFiles {
		relPath, _ := filepath.Rel(moduleDir, file)
		fmt.Printf("  - %s\n", relPath)
	}
	fmt.Println()

	// Validate all files
	result := validateTerraformFiles(fs, tfFiles, schemaFile)

	printModuleValidationResults(result, moduleDir)

	if !result.Passed {
		return fmt.Errorf("module validation failed")
	}

	return nil
}

// ValidateWorkspace implements the validate-workspace command logic
func ValidateWorkspace(fs afero.Fs, workspaceDir, schemaFile string) error {
	fmt.Println("=========================================================")
	fmt.Println("Terranotate - Workspace Validation (Recursive)")
	fmt.Println("=========================================================")
	fmt.Printf("Workspace directory: %s\n", workspaceDir)
	fmt.Printf("Schema file: %s\n\n", schemaFile)

	// Find all Terraform files in the workspace
	tfFiles, err := findWorkspaceTerraformFiles(fs, workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to scan workspace directory: %w", err)
	}

	if len(tfFiles) == 0 {
		return fmt.Errorf("no Terraform files found in workspace: %s", workspaceDir)
	}

	// Group files by directory for better reporting
	filesByDir := groupFilesByDirectory(tfFiles, workspaceDir)

	fmt.Printf("Found %d Terraform files in %d directories:\n", len(tfFiles), len(filesByDir))
	for dir, files := range filesByDir {
		fmt.Printf("\n  📁 %s (%d files)\n", dir, len(files))
		for _, file := range files {
			fmt.Printf("    - %s\n", filepath.Base(file))
		}
	}
	fmt.Println()

	// Validate all files
	result := validateTerraformFiles(fs, tfFiles, schemaFile)

	printWorkspaceValidationResults(result, workspaceDir, filesByDir)

	if !result.Passed {
		return fmt.Errorf("workspace validation failed")
	}

	return nil
}

// Helper functions

func validateModuleStructure(fs afero.Fs, moduleDir string) error {
	info, err := fs.Stat(moduleDir)
	if err != nil {
		return fmt.Errorf("directory does not exist: %s", moduleDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", moduleDir)
	}

	entries, err := afero.ReadDir(fs, moduleDir)
	if err != nil {
		return err
	}

	hasTfFile := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tf") {
			hasTfFile = true
			break
		}
	}

	if !hasTfFile {
		return fmt.Errorf("no .tf files found in module root: %s", moduleDir)
	}

	return nil
}

func findModuleTerraformFiles(fs afero.Fs, moduleDir string) ([]string, error) {
	var tfFiles []string

	// Get files in the root module
	entries, err := afero.ReadDir(fs, moduleDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tf") {
			tfFiles = append(tfFiles, filepath.Join(moduleDir, entry.Name()))
		}
	}

	// Check for modules subdirectory
	modulesDir := filepath.Join(moduleDir, "modules")
	if info, err := fs.Stat(modulesDir); err == nil && info.IsDir() {
		err := afero.Walk(fs, modulesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".tf") {
				tfFiles = append(tfFiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return tfFiles, nil
}

func findWorkspaceTerraformFiles(fs afero.Fs, workspaceDir string) ([]string, error) {
	var tfFiles []string

	err := afero.Walk(fs, workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") ||
				name == "node_modules" ||
				name == ".terraform" ||
				name == "terraform.tfstate.d" {
				return filepath.SkipDir
			}
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".tf") {
			tfFiles = append(tfFiles, path)
		}
		return nil
	})

	return tfFiles, err
}

func groupFilesByDirectory(files []string, baseDir string) map[string][]string {
	result := make(map[string][]string)

	for _, file := range files {
		dir := filepath.Dir(file)
		relDir, _ := filepath.Rel(baseDir, dir)
		if relDir == "." {
			relDir = "root"
		}
		result[relDir] = append(result[relDir], file)
	}

	return result
}

func validateTerraformFiles(fs afero.Fs, files []string, schemaFile string) validator.ValidationResult {
	aggregatedResult := validator.ValidationResult{Passed: true}

	v, err := validator.NewSchemaValidator(fs, schemaFile)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}

	prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
	p := parser.NewCommentParser(fs, prefixes)

	for _, file := range files {
		resources, err := p.ParseFile(file)
		if err != nil {
			log.Printf("Warning: Failed to parse %s: %v", file, err)
			continue
		}

		if len(resources) == 0 {
			continue // Skip files with no resources
		}

		result := v.ValidateResources(resources)

		// Add file context to errors
		for i := range result.Errors {
			result.Errors[i].ResourceType = fmt.Sprintf("%s (%s)",
				result.Errors[i].ResourceType,
				file)
		}

		aggregatedResult.Errors = append(aggregatedResult.Errors, result.Errors...)
		if !result.Passed {
			aggregatedResult.Passed = false
		}
	}

	return aggregatedResult
}

func printModuleValidationResults(result validator.ValidationResult, moduleDir string) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("MODULE VALIDATION RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	if result.Passed {
		fmt.Println("\n✅ Module validation passed!")
		fmt.Printf("   All files in %s meet schema requirements\n", moduleDir)
		return
	}

	fmt.Printf("\n❌ Module validation failed for: %s\n", moduleDir)
	validator.PrintValidationResults(result)
}

func printWorkspaceValidationResults(result validator.ValidationResult, workspaceDir string, filesByDir map[string][]string) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("WORKSPACE VALIDATION RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	if result.Passed {
		fmt.Println("\n✅ Workspace validation passed!")
		fmt.Printf("   All %d directories in %s meet schema requirements\n",
			len(filesByDir), workspaceDir)

		fmt.Println("\n📊 Validated directories:")
		for dir := range filesByDir {
			fmt.Printf("   ✓ %s\n", dir)
		}
		return
	}

	fmt.Printf("\n❌ Workspace validation failed for: %s\n", workspaceDir)

	errorsByDir := make(map[string][]validator.ValidationError)
	for _, err := range result.Errors {
		parts := strings.Split(err.ResourceType, " (")
		if len(parts) == 2 {
			filePath := strings.TrimSuffix(parts[1], ")")
			for dir, files := range filesByDir {
				for _, file := range files {
					if file == filePath {
						// Clean up ResourceType for display
						err.ResourceType = parts[0] + " (" + filepath.Base(filePath) + ")"
						errorsByDir[dir] = append(errorsByDir[dir], err)
						break
					}
				}
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	for dir, errors := range errorsByDir {
		fmt.Printf("\n📁 Directory: %s (%d errors)\n", dir, len(errors))
		fmt.Println(strings.Repeat("-", 80))

		for _, err := range errors {
			severity := "ERROR"
			icon := "❌"
			if err.Severity == "warning" {
				severity = "WARNING"
				icon = "⚠️"
			}

			fmt.Printf("  %s [%s] %s - Line %d\n", icon, severity, err.ResourceType, err.Line)
			fmt.Printf("     %s\n\n", err.Message)
		}
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\nTotal errors: %d across %d directories\n", len(result.Errors), len(errorsByDir))
}
