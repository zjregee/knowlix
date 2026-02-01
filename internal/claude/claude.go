package claude

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"knowlix/internal/models"
)

type Client struct {
	Command []string
	Timeout time.Duration
}

func FromEnv() *Client {
	cmd := os.Getenv("KNOWLIX_CLAUDE_CMD")
	if cmd == "" {
		cmd = os.Getenv("CLAUDE_CODE_CMD")
	}
	if cmd == "" {
		cmd = "claude"
	}
	return &Client{
		Command: SplitCommand(cmd),
		Timeout: 120 * time.Second,
	}
}

func (c *Client) GenerateDescription(item models.ApiItem) (string, error) {
	if len(c.Command) == 0 {
		return "", fmt.Errorf("Claude command is empty")
	}
	prompt := BuildPrompt(item)
	ctx := context.Background()
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.Command[0], c.Command[1:]...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Claude Code failed: %s", strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func BuildPrompt(item models.ApiItem) string {
	header := "You are an expert Go API documentation writer. " +
		"Generate concise, high-quality Markdown documentation in Chinese. " +
		"Focus on what the API does, parameters/returns, and usage notes. " +
		"Do not invent behavior that is not implied by the signature or name. " +
		"Keep it readable and structured.\n"

	lines := []string{
		"Package: " + item.Package,
		"Import: " + item.ImportPath,
		"Kind: " + item.Kind,
		"Name: " + item.Name,
		"Signature: " + item.Signature,
	}
	if item.Receiver != "" {
		lines = append(lines, "Receiver: "+item.Receiver)
	}
	if item.Params != "" {
		lines = append(lines, "Params: "+item.Params)
	}
	if item.Returns != "" {
		lines = append(lines, "Returns: "+item.Returns)
	}
	if item.Kind == "type" {
		lines = append(lines, "TypeKind: "+item.TypeKind)
		if len(item.Fields) > 0 {
			lines = append(lines, "Fields:\n"+strings.Join(item.Fields, "\n"))
		}
		if len(item.Methods) > 0 {
			lines = append(lines, "Methods:\n"+strings.Join(item.Methods, "\n"))
		}
	}
	if item.SourceDescription != "" {
		lines = append(lines, "ExistingDescription: "+item.SourceDescription)
	}

	instructions := "\nPlease output Markdown with these sections when applicable:\n" +
		"- Summary\n" +
		"- Parameters\n" +
		"- Returns\n" +
		"- Notes\n"

	return header + strings.Join(lines, "\n") + instructions
}

func SplitCommand(command string) []string {
	var parts []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch ch {
		case '\\':
			if i+1 < len(command) {
				current.WriteByte(command[i+1])
				i++
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			} else {
				current.WriteByte(ch)
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			} else {
				current.WriteByte(ch)
			}
		case ' ', '\t':
			if inSingle || inDouble {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
