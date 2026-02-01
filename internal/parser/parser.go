package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

type GoFunction struct {
	Name        string
	Signature   string
	Description string
	Receiver    string
	Params      string
	Returns     string
	Package     string
}

type GoType struct {
	Name        string
	Kind        string
	Description string
	Fields      []string
	Methods     []string
	Package     string
}

type GoPackage struct {
	Name        string
	ImportPath  string
	Functions   []GoFunction
	Types       []GoType
	Description string
}

type GoDocParser struct{}

func (p *GoDocParser) ParseRepository(repoPath string) ([]GoPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go list failed: %s", strings.TrimSpace(string(output)))
	}

	packages := []GoPackage{}
	for _, info := range parseGoListJSON(output) {
		importPath := stringValue(info, "ImportPath")
		packageDir := stringValue(info, "Dir")
		docOutput, docErr := runGoDocInDir(packageDir, importPath)
		if docErr != nil {
			return nil, docErr
		}
		functions, types := parseDocOutput(docOutput)
		packages = append(packages, GoPackage{
			Name:        stringValue(info, "Name"),
			ImportPath:  importPath,
			Functions:   functions,
			Types:       types,
			Description: stringValue(info, "Doc"),
		})
	}

	return packages, nil
}

func parseGoListJSON(payload []byte) []map[string]any {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	items := []map[string]any{}
	for {
		var obj map[string]any
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		items = append(items, obj)
	}
	return items
}

func stringValue(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if value, ok := obj[key]; ok {
		if s, ok := value.(string); ok {
			return s
		}
	}
	return ""
}

func runGoDocInDir(packageDir string, importPath string) (string, error) {
	if packageDir != "" {
		cmd := exec.Command("go", "doc", "-all", "./")
		cmd.Dir = packageDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			return string(output), nil
		}
	}
	if importPath == "" {
		return "", nil
	}
	cmd := exec.Command("go", "doc", "-all", importPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go doc failed: %s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

var funcPattern = regexp.MustCompile(`^func(?:\s+\(([^)]+)\))?\s+([A-Z]\w+)\s*(\([^)]*\))?\s*(.*)$`)
var typePattern = regexp.MustCompile(`^type\s+([A-Z]\w+)\s+(struct|interface)\s*\{?`)
var fieldPattern = regexp.MustCompile(`^(\w+)\s+([^\s]+)(?:\s+(.*))?$`)

func parseDocOutput(output string) ([]GoFunction, []GoType) {
	lines := strings.Split(output, "\n")
	functions := []GoFunction{}
	types := []GoType{}
	currentPackage := ""

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")

		if strings.HasPrefix(line, "package ") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				currentPackage = strings.TrimSpace(parts[1])
			}
			continue
		}

		if line == "" || strings.HasPrefix(line, "          ") || strings.HasPrefix(line, "CONSTANTS") || strings.HasPrefix(line, "VARIABLES") {
			continue
		}

		if fn, ok := parseFunction(line); ok {
			fn.Package = currentPackage
			functions = append(functions, fn)
			continue
		}

		if typ, nextIndex, ok := parseType(line, lines, i); ok {
			typ.Package = currentPackage
			types = append(types, typ)
			i = nextIndex - 1
			continue
		}
	}

	return functions, types
}

func parseFunction(line string) (GoFunction, bool) {
	match := funcPattern.FindStringSubmatch(line)
	if match == nil {
		return GoFunction{}, false
	}
	receiver := match[1]
	name := match[2]
	params := match[3]
	returns := match[4]
	if params == "" {
		params = "()"
	}
	description := ""
	if strings.Contains(line, "    ") {
		parts := strings.Split(line, "    ")
		description = strings.TrimSpace(parts[len(parts)-1])
	}
	signature := "func"
	if receiver != "" {
		signature += " (" + receiver + ")"
	}
	signature += " " + name + params
	if returns != "" {
		signature += " " + returns
	}
	return GoFunction{
		Name:        name,
		Signature:   signature,
		Description: description,
		Receiver:    receiver,
		Params:      params,
		Returns:     returns,
	}, true
}

func parseType(line string, lines []string, idx int) (GoType, int, bool) {
	match := typePattern.FindStringSubmatch(line)
	if match == nil {
		return GoType{}, idx, false
	}
	typeName := match[1]
	kind := match[2]

	typ := GoType{
		Name: typeName,
		Kind: kind,
	}

	i := idx + 1
	for i < len(lines) {
		content := lines[i]
		if !strings.HasPrefix(content, " ") && !strings.HasPrefix(content, "\t") {
			break
		}
		content = strings.TrimSpace(content)
		if content == "" || strings.HasPrefix(content, "//") {
			i++
			continue
		}
		if fieldMatch := fieldPattern.FindStringSubmatch(content); fieldMatch != nil {
			fieldStr := content
			if fieldMatch[3] != "" {
				fieldStr = fmt.Sprintf("%s %s // %s", fieldMatch[1], fieldMatch[2], fieldMatch[3])
			}
			typ.Fields = append(typ.Fields, fieldStr)
		} else if strings.HasPrefix(content, "func") {
			typ.Methods = append(typ.Methods, content)
		}
		i++
	}
	return typ, i, true
}
