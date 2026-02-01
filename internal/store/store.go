package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"knowlix/internal/models"
)

type DocStore struct {
	BaseDir string
}

type indexFile struct {
	Repo      string      `json:"repo"`
	Version   string      `json:"version"`
	UpdatedAt string      `json:"updated_at"`
	Items     []indexItem `json:"items"`
}

type indexItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Package     string `json:"package"`
	ImportPath  string `json:"import_path"`
	Signature   string `json:"signature"`
	Path        string `json:"path"`
	GeneratedAt string `json:"generated_at"`
	Generator   string `json:"generator"`
	Model       string `json:"model"`
}

var slugPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (s DocStore) ExistsVersion(repoSlug string, version string, item models.ApiItem) bool {
	_, err := os.Stat(s.docPath(repoSlug, version, item))
	return err == nil
}

func (s DocStore) Upsert(repoSlug string, version string, doc models.GeneratedDoc) (string, error) {
	path := s.docPath(repoSlug, version, doc.Item)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	content := s.renderMarkdown(doc)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	if err := s.updateIndex(repoSlug, version, doc, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s DocStore) docPath(repoSlug string, version string, item models.ApiItem) string {
	packageSlug := safeSlug(item.Package)
	kindSlug := safeSlug(item.Kind)
	nameSlug := safeSlug(itemFilename(item))
	versionSlug := safeSlug(version)
	return filepath.Join(s.BaseDir, repoSlug, versionSlug, packageSlug, kindSlug, fmt.Sprintf("%s.md", nameSlug))
}

func (s DocStore) updateIndex(repoSlug string, version string, doc models.GeneratedDoc, docPath string) error {
	indexPath := filepath.Join(s.BaseDir, repoSlug, safeSlug(version), "index.json")
	index := indexFile{Repo: repoSlug, Version: version, UpdatedAt: utcNow(), Items: []indexItem{}}
	if raw, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(raw, &index)
	}

	relPath, _ := filepath.Rel(s.BaseDir, docPath)
	entry := indexItem{
		ID:          doc.Item.ItemID,
		Kind:        doc.Item.Kind,
		Name:        doc.Item.Name,
		Package:     doc.Item.Package,
		ImportPath:  doc.Item.ImportPath,
		Signature:   doc.Item.Signature,
		Path:        relPath,
		GeneratedAt: doc.GeneratedAt,
		Generator:   doc.Generator,
		Model:       doc.Model,
	}

	updated := false
	for i := range index.Items {
		if index.Items[i].ID == doc.Item.ItemID {
			index.Items[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		index.Items = append(index.Items, entry)
	}
	index.UpdatedAt = utcNow()

	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, encoded, 0o644)
}

func (s DocStore) renderMarkdown(doc models.GeneratedDoc) string {
	frontMatter := []string{
		"---",
		"id: " + doc.Item.ItemID,
		"kind: " + doc.Item.Kind,
		"name: " + doc.Item.Name,
		"package: " + doc.Item.Package,
		"import_path: " + doc.Item.ImportPath,
		"signature: " + doc.Item.Signature,
		"generated_at: " + doc.GeneratedAt,
		"generator: " + doc.Generator,
		"model: " + doc.Model,
		"---",
		"",
	}
	return strings.Join(frontMatter, "\n") + strings.TrimSpace(doc.Content) + "\n"
}

func itemFilename(item models.ApiItem) string {
	if item.Kind == "method" && item.Receiver != "" {
		return item.Receiver + "_" + item.Name
	}
	return item.Name
}

func safeSlug(value string) string {
	if value == "" {
		return "unknown"
	}
	slug := slugPattern.ReplaceAllString(value, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "unknown"
	}
	return slug
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
