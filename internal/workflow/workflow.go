package workflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func NewParser() *WorkflowParser {
	return &WorkflowParser{}
}

func (p *WorkflowParser) FindWorkflowFiles(rootDir string) ([]string, error) {
	workflowsDir := filepath.Join(rootDir, ".github", "workflows")
	return p.FindWorkflowFilesInDir(workflowsDir)
}

func (p *WorkflowParser) FindWorkflowFilesInDir(dir string) ([]string, error) {
	var workflowFiles []string

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return workflowFiles, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")) {
			workflowFiles = append(workflowFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return workflowFiles, nil
}

func (p *WorkflowParser) FindReposWithWorkflows(baseDir string) ([]string, error) {
	var repos []string

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(baseDir, entry.Name())
		workflowsDir := filepath.Join(repoPath, ".github", "workflows")

		if _, err := os.Stat(workflowsDir); err == nil {
			repos = append(repos, repoPath)
		}
	}

	return repos, nil
}

func (p *WorkflowParser) ParseWorkflowFile(filePath string) (*WorkflowFile, error) {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return &WorkflowFile{Path: filePath, Error: err}, err
	}
	defer root.Close()

	f, err := root.Open(base)
	if err != nil {
		return &WorkflowFile{Path: filePath, Error: err}, err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return &WorkflowFile{Path: filePath, Error: err}, err
	}

	uses, err := p.extractUsesFromYAML(content)
	if err != nil {
		return &WorkflowFile{Path: filePath, Uses: uses, Error: err}, err
	}

	usesWithVersions, err := p.extractUsesFromYAMLWithVersions(content)
	if err != nil {
		return &WorkflowFile{Path: filePath, Uses: uses, Error: err}, err
	}

	return &WorkflowFile{Path: filePath, Uses: uses, UsesWithVersions: usesWithVersions}, nil
}

func (p *WorkflowParser) ParseWorkflowFiles(filePaths []string) ([]*WorkflowFile, error) {
	var results []*WorkflowFile

	for _, path := range filePaths {
		workflow, err := p.ParseWorkflowFile(path)
		results = append(results, workflow)
		if err != nil {
			continue
		}
	}

	return results, nil
}

func (p *WorkflowParser) extractUsesFromYAML(content []byte) ([]string, error) {
	var workflow map[string]interface{}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var uses []string
	p.extractUsesRecursive(workflow, &uses)

	uses = p.deduplicateAndClean(uses)

	return uses, nil
}

func (p *WorkflowParser) extractUsesFromYAMLWithVersions(content []byte) ([]ActionRef, error) {
	var workflow map[string]interface{}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var uses []string
	p.extractUsesRecursive(workflow, &uses)

	actionRefs := p.deduplicateAndCleanWithVersions(uses)

	return actionRefs, nil
}

func (p *WorkflowParser) extractUsesRecursive(data interface{}, uses *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "uses" {
				if str, ok := value.(string); ok {
					*uses = append(*uses, str)
				}
			} else {
				p.extractUsesRecursive(value, uses)
			}
		}
	case []interface{}:
		for _, item := range v {
			p.extractUsesRecursive(item, uses)
		}
	}
}

func (p *WorkflowParser) deduplicateAndClean(uses []string) []string {
	seen := make(map[string]bool)
	cleaned := make([]string, 0)

	re := regexp.MustCompile(`^([^/@]+/[^/@]+)@`)

	for _, use := range uses {
		use = strings.TrimSpace(use)
		if use == "" {
			continue
		}

		matches := re.FindStringSubmatch(use)
		if len(matches) > 1 {
			repo := matches[1]
			if !seen[repo] {
				seen[repo] = true
				cleaned = append(cleaned, repo)
			}
		}
	}

	sort.Strings(cleaned)
	return cleaned
}

func (p *WorkflowParser) deduplicateAndCleanWithVersions(uses []string) []ActionRef {
	seen := make(map[string]bool)
	actionRefs := make([]ActionRef, 0)

	re := regexp.MustCompile(`^([^/@]+/[^/@]+)@(.+)$`)

	for _, use := range uses {
		use = strings.TrimSpace(use)
		if use == "" {
			continue
		}

		matches := re.FindStringSubmatch(use)
		if len(matches) > 2 {
			ownerRepo := matches[1]
			version := matches[2]

			if !seen[ownerRepo] {
				seen[ownerRepo] = true
				actionRefs = append(actionRefs, ActionRef{
					OwnerRepo: ownerRepo,
					Version:   version,
					FullRef:   use,
				})
			}
		}
	}

	sort.Slice(actionRefs, func(i, j int) bool {
		return actionRefs[i].OwnerRepo < actionRefs[j].OwnerRepo
	})

	return actionRefs
}

func (p *WorkflowParser) GetAllUsesFromRepo(rootDir string) ([]string, []*WorkflowFile, error) {
	files, err := p.FindWorkflowFiles(rootDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find workflow files: %w", err)
	}

	workflows, err := p.ParseWorkflowFiles(files)
	if err != nil {
		return nil, workflows, fmt.Errorf("failed to parse workflow files: %w", err)
	}

	var allUses []string
	seen := make(map[string]bool)

	for _, workflow := range workflows {
		for _, use := range workflow.Uses {
			if !seen[use] {
				seen[use] = true
				allUses = append(allUses, use)
			}
		}
	}

	sort.Strings(allUses)
	return allUses, workflows, nil
}

func (p *WorkflowParser) GetAllUsesFromRepoWithVersions(rootDir string) ([]ActionRef, []*WorkflowFile, error) {
	files, err := p.FindWorkflowFiles(rootDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find workflow files: %w", err)
	}

	workflows, err := p.ParseWorkflowFiles(files)
	if err != nil {
		return nil, workflows, fmt.Errorf("failed to parse workflow files: %w", err)
	}

	var allActionRefs []ActionRef
	seen := make(map[string]bool)

	for _, workflow := range workflows {
		for _, actionRef := range workflow.UsesWithVersions {
			if !seen[actionRef.OwnerRepo] {
				seen[actionRef.OwnerRepo] = true
				allActionRefs = append(allActionRefs, actionRef)
			}
		}
	}

	sort.Slice(allActionRefs, func(i, j int) bool {
		return allActionRefs[i].OwnerRepo < allActionRefs[j].OwnerRepo
	})

	return allActionRefs, workflows, nil
}
