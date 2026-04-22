// English note.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"cyberstrike-ai/internal/config"

	"gopkg.in/yaml.v3"
)

// English note.
const OrchestratorMarkdownFilename = "orchestrator.md"

// English note.
const OrchestratorPlanExecuteMarkdownFilename = "orchestrator-plan-execute.md"

// English note.
const OrchestratorSupervisorMarkdownFilename = "orchestrator-supervisor.md"

// English note.
type FrontMatter struct {
	Name          string      `yaml:"name"`
	ID            string      `yaml:"id"`
	Description   string      `yaml:"description"`
	Tools         interface{} `yaml:"tools"` //  "A, B"  []string
	MaxIterations int         `yaml:"max_iterations"`
	BindRole      string      `yaml:"bind_role,omitempty"`
	Kind          string      `yaml:"kind,omitempty"` // orchestrator = （ orchestrator.md）
}

// English note.
type OrchestratorMarkdown struct {
	Filename    string
	EinoName    string //  deep.Config.Name / 
	DisplayName string
	Description string
	Instruction string
}

// English note.
type MarkdownDirLoad struct {
	SubAgents               []config.MultiAgentSubConfig
	Orchestrator            *OrchestratorMarkdown // Deep 
	OrchestratorPlanExecute *OrchestratorMarkdown // plan_execute 
	OrchestratorSupervisor  *OrchestratorMarkdown // supervisor 
	FileEntries             []FileAgent           // ， API 
}

// English note.
func OrchestratorMarkdownKind(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	switch {
	case strings.EqualFold(base, OrchestratorPlanExecuteMarkdownFilename):
		return "plan_execute"
	case strings.EqualFold(base, OrchestratorSupervisorMarkdownFilename):
		return "supervisor"
	case strings.EqualFold(base, OrchestratorMarkdownFilename):
		return "deep"
	default:
		return ""
	}
}

// English note.
func IsOrchestratorMarkdown(filename string, fm FrontMatter) bool {
	base := filepath.Base(strings.TrimSpace(filename))
	switch OrchestratorMarkdownKind(base) {
	case "plan_execute", "supervisor":
		return false
	}
	if strings.EqualFold(base, OrchestratorMarkdownFilename) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(fm.Kind), "orchestrator")
}

// English note.
func IsOrchestratorLikeMarkdown(filename string, kind string) bool {
	if OrchestratorMarkdownKind(filename) != "" {
		return true
	}
	return IsOrchestratorMarkdown(filename, FrontMatter{Kind: kind})
}

// English note.
func WantsMarkdownOrchestrator(filename string, kindField string, raw string) bool {
	base := filepath.Base(strings.TrimSpace(filename))
	if OrchestratorMarkdownKind(base) != "" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(kindField), "orchestrator") {
		return true
	}
	if strings.EqualFold(base, OrchestratorMarkdownFilename) {
		return true
	}
	if strings.TrimSpace(raw) == "" {
		return false
	}
	sub, err := ParseMarkdownSubAgent(filename, raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(sub.Kind), "orchestrator")
}

// English note.
func SplitFrontMatter(content string) (frontYAML string, body string, err error) {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, "---") {
		return "", s, nil
	}
	rest := strings.TrimPrefix(s, "---")
	rest = strings.TrimLeft(rest, "\r\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", fmt.Errorf("agents:  --- ")
	}
	fm := strings.TrimSpace(rest[:end])
	body = strings.TrimSpace(rest[end+4:])
	body = strings.TrimLeft(body, "\r\n")
	return fm, body, nil
}

func parseToolsField(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return splitToolList(t)
	case []interface{}:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		var out []string
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func splitToolList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '|'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// English note.
func SlugID(name string) string {
	var b strings.Builder
	name = strings.TrimSpace(strings.ToLower(name))
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '_' || r == '/' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "agent"
	}
	return s
}

// English note.
func sanitizeEinoAgentID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "cyberstrike-deep"
	}
	return out
}

func parseMarkdownAgentRaw(filename string, content string) (FrontMatter, string, error) {
	var fm FrontMatter
	fmStr, body, err := SplitFrontMatter(content)
	if err != nil {
		return fm, "", err
	}
	if strings.TrimSpace(fmStr) == "" {
		return fm, "", fmt.Errorf("agents: %s  YAML front matter", filename)
	}
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return fm, "", fmt.Errorf("agents:  front matter: %w", err)
	}
	return fm, body, nil
}

func orchestratorFromParsed(filename string, fm FrontMatter, body string) (*OrchestratorMarkdown, error) {
	display := strings.TrimSpace(fm.Name)
	if display == "" {
		display = "Orchestrator"
	}
	rawID := strings.TrimSpace(fm.ID)
	if rawID == "" {
		rawID = SlugID(display)
	}
	eino := sanitizeEinoAgentID(rawID)
	return &OrchestratorMarkdown{
		Filename:    filepath.Base(strings.TrimSpace(filename)),
		EinoName:    eino,
		DisplayName: display,
		Description: strings.TrimSpace(fm.Description),
		Instruction: strings.TrimSpace(body),
	}, nil
}

func orchestratorConfigFromOrchestrator(o *OrchestratorMarkdown) config.MultiAgentSubConfig {
	if o == nil {
		return config.MultiAgentSubConfig{}
	}
	return config.MultiAgentSubConfig{
		ID:            o.EinoName,
		Name:          o.DisplayName,
		Description:   o.Description,
		Instruction:   o.Instruction,
		Kind:          "orchestrator",
	}
}

func subAgentFromFrontMatter(filename string, fm FrontMatter, body string) (config.MultiAgentSubConfig, error) {
	var out config.MultiAgentSubConfig
	name := strings.TrimSpace(fm.Name)
	if name == "" {
		return out, fmt.Errorf("agents: %s  name ", filename)
	}
	id := strings.TrimSpace(fm.ID)
	if id == "" {
		id = SlugID(name)
	}
	out.ID = id
	out.Name = name
	out.Description = strings.TrimSpace(fm.Description)
	out.Instruction = strings.TrimSpace(body)
	out.RoleTools = parseToolsField(fm.Tools)
	out.MaxIterations = fm.MaxIterations
	out.BindRole = strings.TrimSpace(fm.BindRole)
	out.Kind = strings.TrimSpace(fm.Kind)
	return out, nil
}

func collectMarkdownBasenames(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("agents: : %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(n), ".md") {
			continue
		}
		if strings.EqualFold(n, "README.md") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// English note.
func LoadMarkdownAgentsDir(dir string) (*MarkdownDirLoad, error) {
	out := &MarkdownDirLoad{}
	names, err := collectMarkdownBasenames(dir)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		p := filepath.Join(dir, n)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		fm, body, err := parseMarkdownAgentRaw(n, string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		switch OrchestratorMarkdownKind(n) {
		case "plan_execute":
			if out.OrchestratorPlanExecute != nil {
				return nil, fmt.Errorf("agents:  %s， %s", OrchestratorPlanExecuteMarkdownFilename, out.OrchestratorPlanExecute.Filename)
			}
			orch, err := orchestratorFromParsed(n, fm, body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n, err)
			}
			out.OrchestratorPlanExecute = orch
			out.FileEntries = append(out.FileEntries, FileAgent{
				Filename:       n,
				Config:         orchestratorConfigFromOrchestrator(orch),
				IsOrchestrator: true,
			})
			continue
		case "supervisor":
			if out.OrchestratorSupervisor != nil {
				return nil, fmt.Errorf("agents:  %s， %s", OrchestratorSupervisorMarkdownFilename, out.OrchestratorSupervisor.Filename)
			}
			orch, err := orchestratorFromParsed(n, fm, body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n, err)
			}
			out.OrchestratorSupervisor = orch
			out.FileEntries = append(out.FileEntries, FileAgent{
				Filename:       n,
				Config:         orchestratorConfigFromOrchestrator(orch),
				IsOrchestrator: true,
			})
			continue
		}
		if IsOrchestratorMarkdown(n, fm) {
			if out.Orchestrator != nil {
				return nil, fmt.Errorf("agents: （Deep ）， %s， %s ", out.Orchestrator.Filename, n)
			}
			orch, err := orchestratorFromParsed(n, fm, body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n, err)
			}
			out.Orchestrator = orch
			out.FileEntries = append(out.FileEntries, FileAgent{
				Filename:       n,
				Config:         orchestratorConfigFromOrchestrator(orch),
				IsOrchestrator: true,
			})
			continue
		}
		sub, err := subAgentFromFrontMatter(n, fm, body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		out.SubAgents = append(out.SubAgents, sub)
		out.FileEntries = append(out.FileEntries, FileAgent{Filename: n, Config: sub, IsOrchestrator: false})
	}
	return out, nil
}

// English note.
func ParseMarkdownSubAgent(filename string, content string) (config.MultiAgentSubConfig, error) {
	fm, body, err := parseMarkdownAgentRaw(filename, content)
	if err != nil {
		return config.MultiAgentSubConfig{}, err
	}
	if OrchestratorMarkdownKind(filename) != "" {
		orch, err := orchestratorFromParsed(filename, fm, body)
		if err != nil {
			return config.MultiAgentSubConfig{}, err
		}
		return orchestratorConfigFromOrchestrator(orch), nil
	}
	if IsOrchestratorMarkdown(filename, fm) {
		orch, err := orchestratorFromParsed(filename, fm, body)
		if err != nil {
			return config.MultiAgentSubConfig{}, err
		}
		return orchestratorConfigFromOrchestrator(orch), nil
	}
	return subAgentFromFrontMatter(filename, fm, body)
}

// English note.
func LoadMarkdownSubAgents(dir string) ([]config.MultiAgentSubConfig, error) {
	load, err := LoadMarkdownAgentsDir(dir)
	if err != nil {
		return nil, err
	}
	return load.SubAgents, nil
}

// English note.
type FileAgent struct {
	Filename       string
	Config         config.MultiAgentSubConfig
	IsOrchestrator bool
}

// English note.
func LoadMarkdownAgentFiles(dir string) ([]FileAgent, error) {
	load, err := LoadMarkdownAgentsDir(dir)
	if err != nil {
		return nil, err
	}
	return load.FileEntries, nil
}

// English note.
func MergeYAMLAndMarkdown(yamlSubs []config.MultiAgentSubConfig, mdSubs []config.MultiAgentSubConfig) []config.MultiAgentSubConfig {
	mdByID := make(map[string]config.MultiAgentSubConfig)
	for _, m := range mdSubs {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		mdByID[id] = m
	}
	yamlIDSet := make(map[string]bool)
	for _, y := range yamlSubs {
		yamlIDSet[strings.TrimSpace(y.ID)] = true
	}
	out := make([]config.MultiAgentSubConfig, 0, len(yamlSubs)+len(mdSubs))
	for _, y := range yamlSubs {
		id := strings.TrimSpace(y.ID)
		if id == "" {
			continue
		}
		if m, ok := mdByID[id]; ok {
			out = append(out, m)
		} else {
			out = append(out, y)
		}
	}
	for _, m := range mdSubs {
		id := strings.TrimSpace(m.ID)
		if id == "" || yamlIDSet[id] {
			continue
		}
		out = append(out, m)
	}
	return out
}

// English note.
func EffectiveSubAgents(yamlSubs []config.MultiAgentSubConfig, agentsDir string) ([]config.MultiAgentSubConfig, error) {
	md, err := LoadMarkdownSubAgents(agentsDir)
	if err != nil {
		return nil, err
	}
	if len(md) == 0 {
		return yamlSubs, nil
	}
	return MergeYAMLAndMarkdown(yamlSubs, md), nil
}

// English note.
func BuildMarkdownFile(sub config.MultiAgentSubConfig) ([]byte, error) {
	fm := FrontMatter{
		Name:          sub.Name,
		ID:            sub.ID,
		Description:   sub.Description,
		MaxIterations: sub.MaxIterations,
		BindRole:      sub.BindRole,
	}
	if k := strings.TrimSpace(sub.Kind); k != "" {
		fm.Kind = k
	}
	if len(sub.RoleTools) > 0 {
		fm.Tools = sub.RoleTools
	}
	head, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(head)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(sub.Instruction))
	if !strings.HasSuffix(sub.Instruction, "\n") && sub.Instruction != "" {
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}
