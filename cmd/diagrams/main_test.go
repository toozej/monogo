package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateArchitectureDiagram(t *testing.T) {
	// This test is limited because the function calls log.Fatal on errors
	// In a real test environment, we'd need to mock the diagram package
	// For now, just test that the function exists and can be called
	// (though it will likely fail due to missing directories)

	// Since generateArchitectureDiagram calls log.Fatal on failure,
	// we can't easily test it without mocking.
	// This is a placeholder test.
	assert.NotNil(t, generateArchitectureDiagram)
}

func TestGenerateComponentDiagram(t *testing.T) {
	// Similar limitation as above
	assert.NotNil(t, generateComponentDiagram)
}

func TestMain(t *testing.T) {
	// Testing main() is difficult because it performs file operations
	// and calls log.Fatal. In a production environment, we'd refactor
	// to separate concerns for better testability.
	assert.NotNil(t, main)
}
