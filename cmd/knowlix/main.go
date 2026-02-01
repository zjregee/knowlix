package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"knowlix/internal/claude"
	"knowlix/internal/models"
	"knowlix/internal/parser"
	"knowlix/internal/repo"
	"knowlix/internal/store"
)

func main() {
	output := flag.String("output", "docs/generated", "Docs output directory")
	claudeCmd := flag.String("claude-cmd", "", "Claude Code command override")
	timeout := flag.Int("timeout", 120, "Claude Code timeout (seconds)")
	force := flag.Bool("force", false, "Regenerate docs even if they exist")
	ref := flag.String("ref", "", "Git ref to checkout (tag/branch/commit)")
	dryRun := flag.Bool("dry-run", false, "Parse and list APIs without generating docs")
	maxItems := flag.Int("max-items", 0, "Maximum number of APIs to process (0 = no limit)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: knowlix <repo> [--ref ref] [--output dir] [--claude-cmd cmd] [--timeout seconds] [--force] [--dry-run] [--max-items n]")
		os.Exit(1)
	}

	repoSource := flag.Arg(0)
	repoSlug := repo.RepoSlugFromSource(repoSource)
	target := repoSource
	cleanup := func() {}
	if repo.IsGitHubRepo(target) {
		var err error
		cloneDepth := 1
		if *ref != "" {
			cloneDepth = 0
		}
		target, cleanup, err = repo.CloneGitHubRepoToTemp(target, cloneDepth)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	defer cleanup()

	if err := repo.CheckoutRef(target, *ref); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	versionKey := repo.VersionKey(target)

	parserClient := &parser.GoDocParser{}
	packages, err := parserClient.ParseRepository(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	items := collectApiItems(packages)
	if *maxItems > 0 && len(items) > *maxItems {
		items = items[:*maxItems]
	}
	if *dryRun {
		for _, item := range items {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\n", versionKey, item.ImportPath, item.Kind, item.Package, item.Signature)
		}
		return
	}
	var client *claude.Client
	if *claudeCmd != "" {
		client = &claude.Client{Command: claude.SplitCommand(*claudeCmd), Timeout: time.Duration(*timeout) * time.Second}
	} else {
		client = claude.FromEnv()
		client.Timeout = time.Duration(*timeout) * time.Second
	}

	docStore := store.DocStore{BaseDir: *output}
	for _, item := range items {
		if !*force && docStore.ExistsVersion(repoSlug, versionKey, item) {
			continue
		}
		content, err := client.GenerateDescription(item)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		doc := models.GeneratedDoc{
			Item:        item,
			Content:     content,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Generator:   "claude-code",
		}
		if _, err := docStore.Upsert(repoSlug, versionKey, doc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func collectApiItems(packages []parser.GoPackage) []models.ApiItem {
	items := []models.ApiItem{}
	for _, pkg := range packages {
		for _, fn := range pkg.Functions {
			kind := "function"
			if fn.Receiver != "" {
				kind = "method"
			}
			itemID := fmt.Sprintf("%s:%s", pkg.ImportPath, fn.Signature)
			items = append(items, models.ApiItem{
				ItemID:            itemID,
				Kind:              kind,
				Name:              fn.Name,
				Signature:         fn.Signature,
				Package:           pkg.Name,
				ImportPath:        pkg.ImportPath,
				Receiver:          fn.Receiver,
				Params:            fn.Params,
				Returns:           fn.Returns,
				SourceDescription: fn.Description,
			})
		}
		for _, typ := range pkg.Types {
			itemID := fmt.Sprintf("%s:type:%s", pkg.ImportPath, typ.Name)
			items = append(items, models.ApiItem{
				ItemID:            itemID,
				Kind:              "type",
				Name:              typ.Name,
				Signature:         fmt.Sprintf("type %s %s", typ.Name, typ.Kind),
				Package:           pkg.Name,
				ImportPath:        pkg.ImportPath,
				TypeKind:          typ.Kind,
				Fields:            typ.Fields,
				Methods:           typ.Methods,
				SourceDescription: typ.Description,
			})
		}
	}
	return items
}
