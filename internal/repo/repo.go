package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var shortRepoPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
var segmentPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func IsGitHubRepo(source string) bool {
	return strings.HasPrefix(source, "https://github.com/") ||
		strings.HasPrefix(source, "http://github.com/") ||
		strings.HasPrefix(source, "git@github.com:") ||
		shortRepoPattern.MatchString(source)
}

func NormalizeGitHubRepo(source string) string {
	if strings.HasPrefix(source, "git@github.com:") {
		return "https://github.com/" + strings.TrimPrefix(source, "git@github.com:")
	}
	if strings.HasPrefix(source, "http://github.com/") {
		return "https://github.com/" + strings.TrimPrefix(source, "http://github.com/")
	}
	if shortRepoPattern.MatchString(source) {
		return "https://github.com/" + source
	}
	return source
}

func RepoSlugFromSource(source string) string {
	if IsGitHubRepo(source) {
		normalized := NormalizeGitHubRepo(source)
		slug := strings.TrimPrefix(normalized, "https://github.com/")
		slug = strings.TrimSuffix(strings.TrimRight(slug, "/"), ".git")
		return strings.ReplaceAll(slug, "/", "_")
	}
	return filepath.Base(source)
}

func CloneGitHubRepoToTemp(source string, depth int) (string, func(), error) {
	repoURL := NormalizeGitHubRepo(source)
	tempDir, err := os.MkdirTemp("", "knowlix-go-")
	if err != nil {
		return "", nil, err
	}
	args := []string{"clone"}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprint(depth))
	}
	args = append(args, repoURL, tempDir)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(output)))
	}
	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}
	return tempDir, cleanup, nil
}

func CheckoutRef(repoPath string, ref string) error {
	if ref == "" {
		return nil
	}
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func VersionKey(repoPath string) string {
	commit, err := headCommit(repoPath)
	if err != nil || commit == "" {
		commit = "unknown"
	}
	tag, _ := headTag(repoPath)
	if tag == "" {
		tag = "untagged"
	}
	return safeSegment(tag) + "-" + commit
}

func headCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func headTag(repoPath string) (string, error) {
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", nil
}

func safeSegment(value string) string {
	if value == "" {
		return "unknown"
	}
	slug := segmentPattern.ReplaceAllString(value, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "unknown"
	}
	return slug
}
