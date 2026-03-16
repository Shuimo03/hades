package prompt

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
	"hades/internal/mcp"
)

//go:embed *.md
var embeddedPromptFS embed.FS

type promptFrontMatter struct {
	Name              string                 `yaml:"name"`
	Title             string                 `yaml:"title"`
	Description       string                 `yaml:"description"`
	ResultDescription string                 `yaml:"result_description"`
	System            string                 `yaml:"system"`
	Role              string                 `yaml:"role"`
	Arguments         []promptArgumentConfig `yaml:"arguments"`
	Messages          []promptMessageConfig  `yaml:"messages"`
}

type promptArgumentConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
}

type promptDocument struct {
	meta            promptFrontMatter
	systemTemplate  *template.Template
	bodyTemplate    *template.Template
	messageTemplate []promptMessageTemplate
}

type promptMessageConfig struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

type promptMessageTemplate struct {
	role     string
	template *template.Template
}

type promptTemplateData struct {
	Args     map[string]string
	Defaults map[string]string
}

func (d promptTemplateData) Arg(name string) string {
	if value := strings.TrimSpace(d.Args[name]); value != "" {
		return value
	}
	return strings.TrimSpace(d.Defaults[name])
}

func RegisterMCPPrompts(server *mcp.HTTPServer) error {
	docs, err := loadPromptDocuments("llm/prompt")
	if err != nil {
		return err
	}

	for _, doc := range docs {
		doc := doc
		server.AddPrompt(buildPromptDefinition(doc.meta), func(_ context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
			return renderPrompt(doc, req)
		})
	}
	return nil
}

func buildPromptDefinition(meta promptFrontMatter) *sdkmcp.Prompt {
	args := make([]*sdkmcp.PromptArgument, 0, len(meta.Arguments))
	for _, arg := range meta.Arguments {
		args = append(args, &sdkmcp.PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}

	return &sdkmcp.Prompt{
		Name:        meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Arguments:   args,
	}
}

func renderPrompt(doc promptDocument, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	args := map[string]string{}
	if req != nil && req.Params != nil && req.Params.Arguments != nil {
		for key, value := range req.Params.Arguments {
			args[key] = value
		}
	}

	defaults := make(map[string]string, len(doc.meta.Arguments))
	for _, arg := range doc.meta.Arguments {
		defaults[arg.Name] = arg.Default
	}

	description := strings.TrimSpace(doc.meta.ResultDescription)
	if description == "" {
		description = doc.meta.Description
	}

	data := promptTemplateData{Args: args, Defaults: defaults}
	messages, err := renderPromptMessages(doc, data)
	if err != nil {
		return nil, err
	}

	return &sdkmcp.GetPromptResult{
		Description: description,
		Messages:    messages,
	}, nil
}

func loadPromptDocuments(dir string) ([]promptDocument, error) {
	if docs, err := loadPromptDocumentsFromOS(dir); err == nil && len(docs) > 0 {
		return docs, nil
	}
	return loadPromptDocumentsFromFS(embeddedPromptFS, ".")
}

func loadPromptDocumentsFromOS(dir string) ([]promptDocument, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	docs := make([]promptDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || isReservedPromptAsset(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read prompt file %s: %w", path, err)
		}
		doc, err := parsePromptDocument(entry.Name(), data)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	sortPromptDocuments(docs)
	return docs, nil
}

func loadPromptDocumentsFromFS(fsys fs.FS, dir string) ([]promptDocument, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}

	docs := make([]promptDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || isReservedPromptAsset(entry.Name()) {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read embedded prompt %s: %w", entry.Name(), err)
		}
		doc, err := parsePromptDocument(entry.Name(), data)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	sortPromptDocuments(docs)
	return docs, nil
}

func sortPromptDocuments(docs []promptDocument) {
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].meta.Name < docs[j].meta.Name
	})
}

func parsePromptDocument(filename string, data []byte) (promptDocument, error) {
	metaText, body, err := splitFrontMatter(string(data))
	if err != nil {
		return promptDocument{}, fmt.Errorf("parse prompt %s: %w", filename, err)
	}

	var meta promptFrontMatter
	if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
		return promptDocument{}, fmt.Errorf("parse prompt %s front matter: %w", filename, err)
	}
	if strings.TrimSpace(meta.Name) == "" {
		return promptDocument{}, fmt.Errorf("parse prompt %s: missing name", filename)
	}
	if strings.TrimSpace(meta.Title) == "" {
		meta.Title = meta.Name
	}
	if strings.TrimSpace(meta.Description) == "" {
		return promptDocument{}, fmt.Errorf("parse prompt %s: missing description", filename)
	}

	doc := promptDocument{meta: meta}
	if strings.TrimSpace(meta.System) != "" {
		tmpl, err := parsePromptTemplate(meta.Name+"_system", meta.System)
		if err != nil {
			return promptDocument{}, fmt.Errorf("parse prompt %s system template: %w", filename, err)
		}
		doc.systemTemplate = tmpl
	}

	for i, message := range meta.Messages {
		if strings.TrimSpace(message.Content) == "" {
			return promptDocument{}, fmt.Errorf("parse prompt %s: empty content for message %d", filename, i)
		}
		tmpl, err := parsePromptTemplate(fmt.Sprintf("%s_message_%d", meta.Name, i), message.Content)
		if err != nil {
			return promptDocument{}, fmt.Errorf("parse prompt %s message template %d: %w", filename, i, err)
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		doc.messageTemplate = append(doc.messageTemplate, promptMessageTemplate{
			role:     role,
			template: tmpl,
		})
	}

	if strings.TrimSpace(body) != "" {
		tmpl, err := parsePromptTemplate(meta.Name+"_body", body)
		if err != nil {
			return promptDocument{}, fmt.Errorf("parse prompt %s body template: %w", filename, err)
		}
		doc.bodyTemplate = tmpl
	}

	if doc.systemTemplate == nil && len(doc.messageTemplate) == 0 && doc.bodyTemplate == nil {
		return promptDocument{}, fmt.Errorf("parse prompt %s: no prompt content found", filename)
	}

	return doc, nil
}

func splitFrontMatter(content string) (string, string, error) {
	trimmed := strings.TrimLeft(content, "\ufeff\r\n\t ")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return "", "", fmt.Errorf("missing YAML front matter")
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return "", "", fmt.Errorf("invalid front matter")
	}

	var metaLines []string
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
		metaLines = append(metaLines, lines[i])
	}
	if endIndex == -1 {
		return "", "", fmt.Errorf("front matter not closed")
	}

	body := strings.Join(lines[endIndex+1:], "\n")
	return strings.Join(metaLines, "\n"), body, nil
}

func parsePromptTemplate(name, content string) (*template.Template, error) {
	return template.New(name).Option("missingkey=zero").Parse(content)
}

func renderPromptMessages(doc promptDocument, data promptTemplateData) ([]*sdkmcp.PromptMessage, error) {
	var messages []*sdkmcp.PromptMessage

	if doc.systemTemplate != nil {
		text, err := executePromptTemplate(doc.meta.Name+"_system_render", doc.systemTemplate, data)
		if err != nil {
			return nil, fmt.Errorf("render prompt %s system: %w", doc.meta.Name, err)
		}
		if text != "" {
			messages = append(messages, newPromptMessage("system", text))
		}
	}

	for i, msg := range doc.messageTemplate {
		text, err := executePromptTemplate(fmt.Sprintf("%s_message_%d_render", doc.meta.Name, i), msg.template, data)
		if err != nil {
			return nil, fmt.Errorf("render prompt %s message %d: %w", doc.meta.Name, i, err)
		}
		if text != "" {
			messages = append(messages, newPromptMessage(msg.role, text))
		}
	}

	if doc.bodyTemplate != nil {
		text, err := executePromptTemplate(doc.meta.Name+"_body_render", doc.bodyTemplate, data)
		if err != nil {
			return nil, fmt.Errorf("render prompt %s body: %w", doc.meta.Name, err)
		}
		if text != "" {
			role := strings.TrimSpace(doc.meta.Role)
			if role == "" {
				role = "user"
			}
			messages = append(messages, newPromptMessage(role, text))
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("render prompt %s: no messages produced", doc.meta.Name)
	}
	return messages, nil
}

func executePromptTemplate(_ string, tmpl *template.Template, data promptTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func newPromptMessage(role, text string) *sdkmcp.PromptMessage {
	return &sdkmcp.PromptMessage{
		Role:    sdkmcp.Role(strings.TrimSpace(role)),
		Content: &sdkmcp.TextContent{Text: text},
	}
}

func isReservedPromptAsset(name string) bool {
	return name == "server_instructions.md"
}
