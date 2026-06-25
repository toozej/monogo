package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsSubPath(t *testing.T) {
	// Get test base paths
	testBase := "/tmp/test/podgrab"
	if runtime.GOOS == "windows" {
		testBase = "C:\\temp\\test\\podgrab"
	}

	tests := []struct {
		name       string
		basePath   string
		targetPath string
		wantErr    bool
	}{
		// 1. Valid subpaths
		{
			name:       "valid direct subpath",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "assets", "podcast.mp3"),
			wantErr:    false,
		},
		{
			name:       "valid nested subpath",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "a", "b", "c", "d", "file.txt"),
			wantErr:    false,
		},

		// 2. Exact base path match
		{
			name:       "exact match of base path",
			basePath:   testBase,
			targetPath: testBase,
			wantErr:    false,
		},

		// 3. Paths with ../ traversal attempts
		{
			name:       "direct ../ traversal",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "..", "etc", "passwd"),
			wantErr:    true,
		},
		{
			name:       "multiple ../ traversal",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "assets", "..", "..", "secret.txt"),
			wantErr:    true,
		},
		{
			name:       "trailing .. traversal",
			basePath:   testBase,
			targetPath: testBase + string(filepath.Separator) + "..",
			wantErr:    true,
		},
		{
			name:       "nested ../ in middle",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "a", "..", "b", "file.mp3"),
			wantErr:    false, // should normalize correctly and still be inside base
		},

		// 4. Absolute paths outside base
		{
			name:       "absolute path outside base",
			basePath:   testBase,
			targetPath: "/etc/hosts",
			wantErr:    true,
		},
		{
			name:       "absolute path at root",
			basePath:   testBase,
			targetPath: string(filepath.Separator),
			wantErr:    true,
		},

		// 5. Edge cases with trailing slashes
		{
			name:       "base with trailing slash, target without",
			basePath:   testBase + string(filepath.Separator),
			targetPath: filepath.Join(testBase, "file.mp3"),
			wantErr:    false,
		},
		{
			name:       "base without trailing slash, target with",
			basePath:   testBase,
			targetPath: filepath.Join(testBase, "dir") + string(filepath.Separator),
			wantErr:    false,
		},
		{
			name:       "both with trailing slashes",
			basePath:   testBase + string(filepath.Separator),
			targetPath: filepath.Join(testBase, "dir") + string(filepath.Separator),
			wantErr:    false,
		},

		// 6. Different path separators (handled via filepath.Abs/Clean)
		{
			name:       "mixed separators in target",
			basePath:   testBase,
			targetPath: testBase + "/dir/file.mp3",
			wantErr:    false,
		},

		// 7. Empty paths
		{
			name:       "empty target path",
			basePath:   testBase,
			targetPath: "",
			wantErr:    true,
		},
		{
			name:       "empty base path",
			basePath:   "",
			targetPath: filepath.Join(testBase, "file.mp3"),
			wantErr:    true,
		},
		{
			name:       "both paths empty",
			basePath:   "",
			targetPath: "",
			wantErr:    false,
		},

		// Additional edge cases
		{
			name:       "same path with extra slashes",
			basePath:   testBase,
			targetPath: testBase + string(filepath.Separator) + string(filepath.Separator) + "file.mp3",
			wantErr:    false,
		},
		{
			name:       "target is parent directory",
			basePath:   filepath.Join(testBase, "subdir"),
			targetPath: testBase,
			wantErr:    true,
		},
		{
			name:       "partial prefix match not allowed",
			basePath:   filepath.Join(testBase, "podcast"),
			targetPath: filepath.Join(testBase, "podcast2", "file.mp3"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isSubPath(tt.basePath, tt.targetPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("isSubPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
