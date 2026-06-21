package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/fixer"
	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
	"gopkg.in/yaml.v3"
)

// Fix implements the fix command logic
func Fix(fs afero.Fs, path, schemaFile string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Auto-Fix Validation Issues")
	fmt.Println("=================================================")
	fmt.Printf("Path: %s\n", path)
	fmt.Printf("Schema file: %s\n\n", schemaFile)

	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	var files []string
	if info.IsDir() {
		files, err = findTerraformFiles(fs, path)
		if err != nil {
			return fmt.Errorf("failed to find terraform files: %w", err)
		}
	} else {
		files = []string{path}
	}

	if len(files) == 0 {
		return fmt.Errorf("no Terraform files found in: %s", path)
	}

	totalFixed := 0
	totalFilesFixed := 0

	for _, file := range files {
		fmt.Printf("\nProcessing: %s\n", file)
		fixed, count, err := fixSingleFile(fs, file, schemaFile)
		if err != nil {
			log.Printf("Warning: Failed to fix %s: %v", file, err)
			continue
		}
		if fixed {
			totalFixed += count
			totalFilesFixed++
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Fix Summary: %d files processed, %d files fixed, %d total fixes applied\n", len(files), totalFilesFixed, totalFixed)
	fmt.Println(strings.Repeat("=", 50))

	return nil
}

func fixSingleFile(fs afero.Fs, terraformFile, schemaFile string) (bool, int, error) {
	// Parse the Terraform file
	prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
	p := parser.NewCommentParser(fs, prefixes)

	resources, err := p.ParseFile(terraformFile)
	if err != nil {
		return false, 0, fmt.Errorf("failed to parse Terraform file: %w", err)
	}

	// Load and validate against schema
	v, err := validator.NewSchemaValidator(fs, schemaFile)
	if err != nil {
		return false, 0, fmt.Errorf("failed to load schema: %w", err)
	}

	fmt.Println("  Analyzing validation errors...")
	result := v.ValidateResources(resources)

	if result.Passed {
		fmt.Println("  ✅ No issues found - file already passes validation!")
		return false, 0, nil
	}

	fmt.Printf("  Found %d validation errors\n", len(result.Errors))
	fmt.Println("  Attempting to fix issues...")

	// Create backup
	backupFile := terraformFile + ".bak"
	if err := fixer.CopyFile(fs, terraformFile, backupFile); err != nil {
		return false, 0, fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("  ✅ Created backup: %s\n", backupFile)

	// Load schema for fixer
	schema, err := loadSchema(fs, schemaFile)
	if err != nil {
		return false, 0, fmt.Errorf("failed to parse schema for fixer: %w", err)
	}

	// Fix the file
	f := fixer.NewCommentFixer(fs, schema)
	fixedContent, fixCount, err := f.FixFile(terraformFile, resources, result.Errors)
	if err != nil {
		return false, 0, fmt.Errorf("failed to fix file: %w", err)
	}

	// Write fixed content
	// #nosec G306 - Writing source code (Terraform), 0644 is appropriate
	// Using afero abstraction
	if err := afero.WriteFile(fs, terraformFile, []byte(fixedContent), 0644); err != nil {
		return false, 0, fmt.Errorf("failed to write fixed file: %w", err)
	}

	fmt.Printf("  ✅ Applied %d fixes to %s\n", fixCount, terraformFile)
	fmt.Println("  Re-validating fixed file...")

	// Re-validate
	resources, _ = p.ParseFile(terraformFile)
	newResult := v.ValidateResources(resources)

	if newResult.Passed {
		fmt.Println("  ✅ All fixable issues resolved! File now passes validation.")
	} else {
		fmt.Printf("  ⚠️  %d issues remain (may require manual intervention)\n", len(newResult.Errors))
		// Optional: print detailed remaining errors
	}

	fmt.Printf("  💡 Backup saved as: %s\n", backupFile)
	return true, fixCount, nil
}

func loadSchema(fs afero.Fs, schemaFile string) (validator.ValidationSchema, error) {
	var schema validator.ValidationSchema
	// #nosec G304 - Schema file provided by user
	data, err := afero.ReadFile(fs, schemaFile)
	if err != nil {
		return schema, err
	}

	if err := yaml.Unmarshal(data, &schema); err != nil {
		return schema, err
	}

	return schema, nil
}

func findTerraformFiles(fs afero.Fs, root string) ([]string, error) {
	var files []string
	err := afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tf") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// RevertFix reverts files to their backup versions
func RevertFix(fs afero.Fs, path string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Revert to Backup Files")
	fmt.Println("=================================================")
	fmt.Printf("Path: %s\n\n", path)

	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	var filesToRevert []string
	if info.IsDir() {
		// Find all .bak files in the directory
		err := afero.Walk(fs, path, func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(file, ".bak") {
				filesToRevert = append(filesToRevert, file)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to find backup files: %w", err)
		}
	} else {
		// Single file - check if corresponding .bak exists
		backupFile := path + ".bak"
		exists, err := afero.Exists(fs, backupFile)
		if err != nil {
			return fmt.Errorf("failed to check for backup file: %w", err)
		}
		if exists {
			filesToRevert = append(filesToRevert, backupFile)
		}
	}

	if len(filesToRevert) == 0 {
		fmt.Println("No backup files found to revert.")
		return nil
	}

	fmt.Printf("Found %d backup file(s) to revert.\n\n", len(filesToRevert))

	revertCount := 0
	for _, backupFile := range filesToRevert {
		originalFile := strings.TrimSuffix(backupFile, ".bak")
		fmt.Printf("Reverting: %s\n", originalFile)

		// Copy backup to original
		if err := fixer.CopyFile(fs, backupFile, originalFile); err != nil {
			log.Printf("  ⚠️  Warning: Failed to revert %s: %v", originalFile, err)
			continue
		}

		// Remove backup file
		if err := fs.Remove(backupFile); err != nil {
			log.Printf("  ⚠️  Warning: Failed to remove backup %s: %v", backupFile, err)
			continue
		}

		fmt.Printf("  ✅ Reverted %s\n", originalFile)
		revertCount++
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Revert Summary: %d file(s) reverted successfully\n", revertCount)
	fmt.Println(strings.Repeat("=", 50))

	return nil
}
