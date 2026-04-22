package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version     string                `yaml:"version,omitempty" json:"version,omitempty"` // ?띸ク?양ㅊ?꾤뎵?у뤇뚦쫩 v1.3.3
	Server      ServerConfig          `yaml:"server"`
	Log         LogConfig             `yaml:"log"`
	MCP         MCPConfig             `yaml:"mcp"`
	OpenAI      OpenAIConfig          `yaml:"openai"`
	FOFA        FofaConfig            `yaml:"fofa,omitempty" json:"fofa,omitempty"`
	Agent       AgentConfig           `yaml:"agent"`
	Security    SecurityConfig        `yaml:"security"`
	Database    DatabaseConfig        `yaml:"database"`
	Auth        AuthConfig            `yaml:"auth"`
	ExternalMCP ExternalMCPConfig     `yaml:"external_mcp,omitempty"`
	Knowledge   KnowledgeConfig       `yaml:"knowledge,omitempty"`
	Robots      RobotsConfig          `yaml:"robots,omitempty" json:"robots,omitempty"`         // 곦툣?에/?됮뭺/욂묘됪쑛?ⓧ볶?띸쉰
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // 믦돯?띸쉰?뉏뻑Total퐬덃뼭?밧폀?
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // ?묈릮?쇔?싨뵱?곩쑉삯뀓?뻼뜸릎싦퉱믦돯
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills?띸쉰?뉏뻑Total퐬
	AgentsDir   string                `yaml:"agents_dir,omitempty" json:"agents_dir,omitempty"` // 싦빰?녶춴 Agent Markdown 싦퉱Total퐬?.md똜AML front matter?
	MultiAgent  MultiAgentConfig      `yaml:"multi_agent,omitempty" json:"multi_agent,omitempty"`
}

// English note.
type MultiAgentConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	DefaultMode        string `yaml:"default_mode" json:"default_mode"`                   // single | multi뚥풘?띸ク섋?뺟ㅊ
	RobotUseMultiAgent bool   `yaml:"robot_use_multi_agent" json:"robot_use_multi_agent"` // ?true ?띌뭺Total욂묘/곩쒜?뷴솳븃뎔 Eino 싦빰Total
	BatchUseMultiAgent bool   `yaml:"batch_use_multi_agent" json:"batch_use_multi_agent"` // ?true ?뜻돶?뤶뻣?↓삜?쀤릎뤷춴삣뒦?Eino 싦빰Total
	// English note.
	Orchestration string `yaml:"orchestration,omitempty" json:"orchestration,omitempty"`
	MaxIteration  int    `yaml:"max_iteration" json:"max_iteration"` // 삡빰Total/ ?㎬죱?ⓩ?㎪렓?녻쉰∽펷Deep?갨upervisor?걈lan_execute TotalExecutor?
	// English note.
	PlanExecuteLoopMaxIterations int                   `yaml:"plan_execute_loop_max_iterations,omitempty" json:"plan_execute_loop_max_iterations,omitempty"`
	SubAgentMaxIterations        int                   `yaml:"sub_agent_max_iterations" json:"sub_agent_max_iterations"`
	WithoutGeneralSubAgent       bool                  `yaml:"without_general_sub_agent" json:"without_general_sub_agent"`
	WithoutWriteTodos            bool                  `yaml:"without_write_todos" json:"without_write_todos"`
	OrchestratorInstruction      string                `yaml:"orchestrator_instruction" json:"orchestrator_instruction"`
	// English note.
	OrchestratorInstructionPlanExecute string `yaml:"orchestrator_instruction_plan_execute,omitempty" json:"orchestrator_instruction_plan_execute,omitempty"`
	// English note.
	OrchestratorInstructionSupervisor string `yaml:"orchestrator_instruction_supervisor,omitempty" json:"orchestrator_instruction_supervisor,omitempty"`
	SubAgents                    []MultiAgentSubConfig `yaml:"sub_agents" json:"sub_agents"`
	// EinoSkills configures CloudWeGo Eino ADK skill middleware + optional local filesystem/execute on DeepAgent.
	EinoSkills MultiAgentEinoSkillsConfig `yaml:"eino_skills,omitempty" json:"eino_skills,omitempty"`
	// EinoMiddleware wires optional ADK middleware (patchtoolcalls, toolsearch, plantask, reduction) and Deep extras.
	EinoMiddleware MultiAgentEinoMiddlewareConfig `yaml:"eino_middleware,omitempty" json:"eino_middleware,omitempty"`
}

// MultiAgentEinoMiddlewareConfig optional Eino ADK middleware and Deep / supervisor tuning.
type MultiAgentEinoMiddlewareConfig struct {
	// PatchToolCalls inserts placeholder tool results for dangling assistant tool_calls (nil = enabled).
	PatchToolCalls *bool `yaml:"patch_tool_calls,omitempty" json:"patch_tool_calls,omitempty"`
	// ToolSearch enables dynamictool/toolsearch: hide tail tools until model calls tool_search (reduces prompt tools).
	ToolSearchEnable        bool `yaml:"tool_search_enable,omitempty" json:"tool_search_enable,omitempty"`
	ToolSearchMinTools      int  `yaml:"tool_search_min_tools,omitempty" json:"tool_search_min_tools,omitempty"`           // default 20; applies when len(tools) >= this
	ToolSearchAlwaysVisible int  `yaml:"tool_search_always_visible,omitempty" json:"tool_search_always_visible,omitempty"` // default 12; first N tools stay always visible
	// Plantask adds TaskCreate/Get/Update/List (file-backed under skills dir); requires eino_skills + local backend.
	PlantaskEnable bool `yaml:"plantask_enable,omitempty" json:"plantask_enable,omitempty"`
	// PlantaskRelDir relative to skills_dir for per-conversation task boards (default .eino/plantask).
	PlantaskRelDir string `yaml:"plantask_rel_dir,omitempty" json:"plantask_rel_dir,omitempty"`
	// Reduction truncates/offloads large tool outputs (requires eino local backend for Write).
	ReductionEnable           bool     `yaml:"reduction_enable,omitempty" json:"reduction_enable,omitempty"`
	ReductionRootDir          string   `yaml:"reduction_root_dir,omitempty" json:"reduction_root_dir,omitempty"` // default: os temp + conversation id
	ReductionClearExclude     []string `yaml:"reduction_clear_exclude,omitempty" json:"reduction_clear_exclude,omitempty"`
	ReductionSubAgents        bool     `yaml:"reduction_sub_agents,omitempty" json:"reduction_sub_agents,omitempty"` // also attach to sub-agents
	// CheckpointDir when non-empty enables adk.Runner CheckPointStore (file-backed) for interrupt/resume persistence.
	CheckpointDir string `yaml:"checkpoint_dir,omitempty" json:"checkpoint_dir,omitempty"`
	// DeepOutputKey passed to deep.Config OutputKey (session final text); empty = off.
	DeepOutputKey string `yaml:"deep_output_key,omitempty" json:"deep_output_key,omitempty"`
	// DeepModelRetryMaxRetries > 0 enables deep.Config ModelRetryConfig (framework-level chat model retries).
	DeepModelRetryMaxRetries int `yaml:"deep_model_retry_max_retries,omitempty" json:"deep_model_retry_max_retries,omitempty"`
	// TaskToolDescriptionPrefix when non-empty sets deep.Config TaskToolDescriptionGenerator (sub-agent names appended).
	TaskToolDescriptionPrefix string `yaml:"task_tool_description_prefix,omitempty" json:"task_tool_description_prefix,omitempty"`
}

// MultiAgentEinoSkillsConfig toggles Eino official skill progressive disclosure and host filesystem tools.
type MultiAgentEinoSkillsConfig struct {
	// Disable skips skill middleware (and does not attach local FS tools for Deep).
	Disable bool `yaml:"disable" json:"disable"`
	// FilesystemTools registers read_file/glob/grep/write/edit/execute (eino-ext local backend). Nil/omitted = true.
	FilesystemTools *bool `yaml:"filesystem_tools,omitempty" json:"filesystem_tools,omitempty"`
	// SkillToolName overrides the default Eino tool name "skill".
	SkillToolName string `yaml:"skill_tool_name,omitempty" json:"skill_tool_name,omitempty"`
}

// EinoSkillFilesystemToolsEffective returns whether Deep/sub-agents should attach local filesystem + streaming shell.
func (c MultiAgentEinoSkillsConfig) EinoSkillFilesystemToolsEffective() bool {
	if c.FilesystemTools != nil {
		return *c.FilesystemTools
	}
	return true
}

// PatchToolCallsEffective returns whether patchtoolcalls middleware should run (default true).
func (c MultiAgentEinoMiddlewareConfig) PatchToolCallsEffective() bool {
	if c.PatchToolCalls != nil {
		return *c.PatchToolCalls
	}
	return true
}

// English note.
type MultiAgentSubConfig struct {
	ID            string   `yaml:"id" json:"id"`
	Name          string   `yaml:"name" json:"name"`
	Description   string   `yaml:"description" json:"description"`
	Instruction   string   `yaml:"instruction" json:"instruction"`
	BindRole      string   `yaml:"bind_role,omitempty" json:"bind_role,omitempty"` // Total됵폏?녘걫삯뀓?roles ?쉪믦돯?랃폑?ら뀓 role_tools ?뜻꼬?②?믦돯Totaltools뚦뭉Totalskills ?쇿뀯?뉏빱?먪ㅊ
	RoleTools     []string `yaml:"role_tools" json:"role_tools"`                   // 롥뜒 Agent 믦돯ε끁?멨릪 key쏁㈉①ㅊ?③깿ε끁늒ind_role Total‥Totaltools?
	MaxIterations int      `yaml:"max_iterations" json:"max_iterations"`
	Kind          string   `yaml:"kind,omitempty" json:"kind,omitempty"` // ?Markdown쉓ind=orchestrator ①ㅊ Deep 삡빰?놅펷?orchestrator.md 뚪됦??츣?
}

// English note.
type MultiAgentPublic struct {
	Enabled                      bool   `json:"enabled"`
	DefaultMode                  string `json:"default_mode"`
	RobotUseMultiAgent           bool   `json:"robot_use_multi_agent"`
	BatchUseMultiAgent           bool   `json:"batch_use_multi_agent"`
	SubAgentCount                int    `json:"sub_agent_count"`
	Orchestration                string `json:"orchestration,omitempty"`
	PlanExecuteLoopMaxIterations int    `json:"plan_execute_loop_max_iterations"`
}

// English note.
func NormalizeMultiAgentOrchestration(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "plan_execute", "plan-execute", "planexecute", "pe":
		return "plan_execute"
	case "supervisor", "super", "sv":
		return "supervisor"
	default:
		return "deep"
	}
}

// English note.
type MultiAgentAPIUpdate struct {
	Enabled                      bool   `json:"enabled"`
	DefaultMode                  string `json:"default_mode"`
	RobotUseMultiAgent           bool   `json:"robot_use_multi_agent"`
	BatchUseMultiAgent           bool   `json:"batch_use_multi_agent"`
	PlanExecuteLoopMaxIterations *int   `json:"plan_execute_loop_max_iterations,omitempty"`
}

// English note.
type RobotsConfig struct {
	Wecom    RobotWecomConfig    `yaml:"wecom,omitempty" json:"wecom,omitempty"`       // 곦툣?에
	Dingtalk RobotDingtalkConfig `yaml:"dingtalk,omitempty" json:"dingtalk,omitempty"` // ?됮뭺
	Lark     RobotLarkConfig     `yaml:"lark,omitempty" json:"lark,omitempty"`         // 욂묘
}

// English note.
type RobotWecomConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Token          string `yaml:"token" json:"token"`                       // ?욆컘 URL ?↓챿 Token
	EncodingAESKey string `yaml:"encoding_aes_key" json:"encoding_aes_key"` // EncodingAESKey
	CorpID         string `yaml:"corp_id" json:"corp_id"`                   // 곦툣 ID
	Secret         string `yaml:"secret" json:"secret"`                     // 붺뵪 Secret
	AgentID        int64  `yaml:"agent_id" json:"agent_id"`                 // 붺뵪 AgentId
}

// English note.
type RobotDingtalkConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ClientID     string `yaml:"client_id" json:"client_id"`         // 붺뵪 Key (AppKey)
	ClientSecret string `yaml:"client_secret" json:"client_secret"` // 붺뵪 Secret
}

// English note.
type RobotLarkConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	AppID       string `yaml:"app_id" json:"app_id"`             // 붺뵪 App ID
	AppSecret   string `yaml:"app_secret" json:"app_secret"`     // 붺뵪 App Secret
	VerifyToken string `yaml:"verify_token" json:"verify_token"` // 뗤뻑?쁾 Verification Token덂룾?됵펹
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type MCPConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	AuthHeader      string `yaml:"auth_header,omitempty"`       // ?닸쓢 header ?랃펽?숂㈉①ㅊ띺돱Total
	AuthHeaderValue string `yaml:"auth_header_value,omitempty"` // ?닸쓢 header ?쇽펽?롨?귚릎?header Total
}

type OpenAIConfig struct {
	Provider       string `yaml:"provider,omitempty" json:"provider,omitempty"` // API ?먧풘Total "openai"(섋?) Total"claude"똠laude ?띈눎?ⓩ‥?δ맏 Anthropic Messages API
	APIKey         string `yaml:"api_key" json:"api_key"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	Model          string `yaml:"model" json:"model"`
	MaxTotalTokens int    `yaml:"max_total_tokens,omitempty" json:"max_total_tokens,omitempty"`
}

type FofaConfig struct {
	// English note.
	Email   string `yaml:"email,omitempty" json:"email,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"` // 섋? https://fofa.info/api/v1/search/all
}

type SecurityConfig struct {
	Tools               []ToolConfig `yaml:"tools,omitempty"`                 // ?묈릮?쇔?싨뵱?곩쑉삯뀓?뻼뜸릎싦퉱ε끁
	ToolsDir            string       `yaml:"tools_dir,omitempty"`             // ε끁?띸쉰?뉏뻑Total퐬덃뼭?밧폀?
	ToolDescriptionMode string       `yaml:"tool_description_mode,omitempty"` // ε끁?뤺염▼폀: "short" | "full"뚪퍡?short
	ActionEnabled       bool         `yaml:"action_enabled,omitempty" json:"action_enabled,omitempty"`
}

type DatabaseConfig struct {
	Path            string `yaml:"path"`                        // 싪캕?경뜮볢러?
	KnowledgeDBPath string `yaml:"knowledge_db_path,omitempty"` // ?θ칳볠빊Total틩?푶덂룾?됵펽븀㈉?쇾슴?ⓧ폏앮빊Total틩?
}

type AgentConfig struct {
	MaxIterations        int    `yaml:"max_iterations" json:"max_iterations"`
	LargeResultThreshold int    `yaml:"large_result_threshold" json:"large_result_threshold"` // ㎫퍜?쒒삁?쇽펷쀨뒄됵펽섋?50KB
	ResultStorageDir     string `yaml:"result_storage_dir" json:"result_storage_dir"`         // 볠옖섇궓Total퐬뚪퍡쨟mp
	ToolTimeoutMinutes   int    `yaml:"tool_timeout_minutes" json:"tool_timeout_minutes"`     // ?뺞Аε끁?㎬죱?㎪뿶?울펷?녽뮓됵펽끾뿶?ゅ뒯덃?뚪삻?빣?띌뿴?귟돈? ①ㅊ띺솏?띰펷띷렓?먲펹
	// English note.
	SystemPromptPath string `yaml:"system_prompt_path,omitempty" json:"system_prompt_path,omitempty"`
}

type AuthConfig struct {
	Password                    string `yaml:"password" json:"password"`
	SessionDurationHours        int    `yaml:"session_duration_hours" json:"session_duration_hours"`
	GeneratedPassword           string `yaml:"-" json:"-"`
	GeneratedPasswordPersisted  bool   `yaml:"-" json:"-"`
	GeneratedPasswordPersistErr string `yaml:"-" json:"-"`
}

// English note.
type ExternalMCPConfig struct {
	Servers map[string]ExternalMCPServerConfig `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// English note.
type ExternalMCPServerConfig struct {
	// English note.
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"` // Total쥊?섌뇧덄뵪럖tdio▼폀?

	// English note.
	Transport string            `yaml:"transport,omitempty" json:"transport,omitempty"` // "stdio" | "sse" | "http"(Streamable) | "simple_http"(?ゅ뻠/?뷥OST?궧뚦쫩?ф쑛 http://127.0.0.1:8081/mcp)
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // HTTP/SSE 룡콆댐펷?x-api-key?

	// English note.
	Description       string          `yaml:"description,omitempty" json:"description,omitempty"`
	Timeout           int             `yaml:"timeout,omitempty" json:"timeout,omitempty"`                         // 끾뿶?띌뿴덄쭜?
	ExternalMCPEnable bool            `yaml:"external_mcp_enable,omitempty" json:"external_mcp_enable,omitempty"` // Total맔Total뵪뽭깿MCP
	ToolEnabled       map[string]bool `yaml:"tool_enabled,omitempty" json:"tool_enabled,omitempty"`               // 뤶릉ε끁?꾢맦?①듁?곻펷ε끁?띸㎞ -> Total맔Total뵪?

	// English note.
	Enabled  bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`   // 꿨틹껓펽욜뵪 external_mcp_enable
	Disabled bool `yaml:"disabled,omitempty" json:"disabled,omitempty"` // 꿨틹껓펽욜뵪 external_mcp_enable
}
type ToolConfig struct {
	Name             string            `yaml:"name"`
	Command          string            `yaml:"command"`
	Args             []string          `yaml:"args,omitempty"`              // ?뷴츣?귝빊덂룾?됵펹
	ShortDescription string            `yaml:"short_description,omitempty"` // Total룒곤펷?ⓧ틢ε끁?쀨〃뚦뇧몋oken덅쀯펹
	Description      string            `yaml:"description"`                 // ?퍏?뤺염덄뵪롥램?룡뻼ｏ펹
	Enabled          bool              `yaml:"enabled"`
	RequiresActionEnabled bool         `yaml:"requires_action_enabled,omitempty" json:"requires_action_enabled,omitempty"`
	Parameters       []ParameterConfig `yaml:"parameters,omitempty"`         // ?귝빊싦퉱덂룾?됵펹
	ArgMapping       string            `yaml:"arg_mapping,omitempty"`        // ?귝빊?졾컙?밧폀: "auto", "manual", "template"덂룾?됵펹
	AllowedExitCodes []int             `yaml:"allowed_exit_codes,omitempty"` // ?곮Total꾦?븀쟻?쀨〃덃윇쎾램?룟쑉?먨뒣?뜸튋붷썮?욇쎏??븀쟻?
}

// English note.
type ParameterConfig struct {
	Name        string      `yaml:"name"`                // ?귝빊?띸㎞
	Type        string      `yaml:"type"`                // ?귝빊삣엹: string, int, bool, array
	Description string      `yaml:"description"`         // ?귝빊?뤺염
	Required    bool        `yaml:"required,omitempty"`  // Total맔낂?
	Default     interface{} `yaml:"default,omitempty"`   // 섋Offset
	ItemType    string      `yaml:"item_type,omitempty"` // ?type ?array ?띰펽?곁퍍?껆킔삣엹뚦쫩 string, number, object
	Flag        string      `yaml:"flag,omitempty"`      // ?썰빱뚧젃쀯펽?"-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"`  // 띸쉰?귝빊?꾡퐤?펷?뗰펹
	Format      string      `yaml:"format,omitempty"`    // ?귝빊?쇔폀: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"`  // →씮쀧Е뀐펽?"{flag} {value}" Total"{value}"
	Options     []string    `yaml:"options,omitempty"`   // Total됧쇔닓⑨펷?ⓧ틢?싦맘?
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Auth.SessionDurationHours <= 0 {
		cfg.Auth.SessionDurationHours = 12
	}

	if strings.TrimSpace(cfg.Auth.Password) == "" {
		password, err := generateStrongPassword(24)
		if err != nil {
			return nil, fmt.Errorf("failed to generate a password: %w", err)
		}

		cfg.Auth.Password = password
		cfg.Auth.GeneratedPassword = password

		if err := PersistAuthPassword(path, password); err != nil {
			cfg.Auth.GeneratedPasswordPersisted = false
			cfg.Auth.GeneratedPasswordPersistErr = err.Error()
		} else {
			cfg.Auth.GeneratedPasswordPersisted = true
		}
	}

	// English note.
	if cfg.Security.ToolsDir != "" {
		configDir := filepath.Dir(path)
		toolsDir := cfg.Security.ToolsDir

		// English note.
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		tools, err := LoadToolsFromDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load tools from directory: %w", err)
		}

		// English note.
		existingTools := make(map[string]bool)
		for _, tool := range tools {
			existingTools[tool.Name] = true
		}

		// English note.
		for _, tool := range cfg.Security.Tools {
			if !existingTools[tool.Name] {
				tools = append(tools, tool)
			}
		}

		cfg.Security.Tools = tools
	}

	// English note.
	if cfg.ExternalMCP.Servers != nil {
		for name, serverCfg := range cfg.ExternalMCP.Servers {
			// English note.
			// English note.
			// English note.
			// English note.
			if serverCfg.Disabled {
				// English note.
				serverCfg.ExternalMCPEnable = false
			} else if serverCfg.Enabled {
				// English note.
				serverCfg.ExternalMCPEnable = true
			} else {
				// English note.
				serverCfg.ExternalMCPEnable = true
			}
			cfg.ExternalMCP.Servers[name] = serverCfg
		}
	}

	// English note.
	if cfg.RolesDir != "" {
		configDir := filepath.Dir(path)
		rolesDir := cfg.RolesDir

		// English note.
		if !filepath.IsAbs(rolesDir) {
			rolesDir = filepath.Join(configDir, rolesDir)
		}

		roles, err := LoadRolesFromDir(rolesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load roles from directory: %w", err)
		}

		cfg.Roles = roles
	} else {
		// English note.
		if cfg.Roles == nil {
			cfg.Roles = make(map[string]RoleConfig)
		}
	}

	return &cfg, nil
}

func generateStrongPassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}

	bytesLen := length
	randomBytes := make([]byte, bytesLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	password := base64.RawURLEncoding.EncodeToString(randomBytes)
	if len(password) > length {
		password = password[:length]
	}
	return password, nil
}

func PersistAuthPassword(path, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	inAuthBlock := false
	authIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inAuthBlock {
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= authIndent {
			// English note.
			inAuthBlock = false
			authIndent = -1
			// English note.
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = leadingSpaces
			}
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "password:") {
			prefix := line[:len(line)-len(strings.TrimLeft(line, " "))]
			comment := ""
			if idx := strings.Index(line, "#"); idx >= 0 {
				comment = strings.TrimRight(line[idx:], " ")
			}

			newLine := fmt.Sprintf("%spassword: %s", prefix, password)
			if comment != "" {
				if !strings.HasPrefix(comment, " ") {
					newLine += " "
				}
				newLine += comment
			}
			lines[i] = newLine
			break
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func PrintGeneratedPasswordWarning(password string, persisted bool, persistErr string) {
	if strings.TrimSpace(password) == "" {
		return
	}

	if persisted {
		fmt.Println("[CyberStrikeAI] A web password was generated and saved to config.yaml.")
	} else {
		if persistErr != "" {
			fmt.Printf("[CyberStrikeAI] A web password was generated, but saving it to config.yaml failed: %s\n", persistErr)
		} else {
			fmt.Println("[CyberStrikeAI] A web password was generated, but saving it to config.yaml failed.")
		}
		fmt.Println("Please update auth.password in config.yaml manually.")
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("CyberStrikeAI Auto-Generated Web Password")
	fmt.Printf("Password: %s\n", password)
	fmt.Println("WARNING: Anyone with this password can fully control CyberStrikeAI.")
	fmt.Println("Please store it securely and change it in config.yaml as soon as possible.")
	fmt.Println("Keep this password private and rotate it after the first login.")
	fmt.Println("You can change it later by editing auth.password in config.yaml.")
	fmt.Println("----------------------------------------------------------------")
}

// English note.
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// English note.
func persistMCPAuth(path string, mcp *MCPConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inMcpBlock := false
	mcpIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inMcpBlock {
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= mcpIndent {
			inMcpBlock = false
			mcpIndent = -1
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = leadingSpaces
			}
			continue
		}

		prefix := line[:leadingSpaces]
		rest := strings.TrimSpace(line[leadingSpaces:])
		comment := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			comment = strings.TrimRight(line[idx:], " ")
		}
		withComment := ""
		if comment != "" {
			if !strings.HasPrefix(comment, " ") {
				withComment = " "
			}
			withComment += comment
		}

		if strings.HasPrefix(rest, "auth_header_value:") {
			lines[i] = fmt.Sprintf("%sauth_header_value: %q%s", prefix, mcp.AuthHeaderValue, withComment)
		} else if strings.HasPrefix(rest, "auth_header:") {
			lines[i] = fmt.Sprintf("%sauth_header: %q%s", prefix, mcp.AuthHeader, withComment)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// English note.
func EnsureMCPAuth(path string, cfg *Config) error {
	if !cfg.MCP.Enabled || strings.TrimSpace(cfg.MCP.AuthHeaderValue) != "" {
		return nil
	}
	token, err := generateRandomToken()
	if err != nil {
		return fmt.Errorf("failed to generate MCP auth token: %w", err)
	}
	cfg.MCP.AuthHeaderValue = token
	if strings.TrimSpace(cfg.MCP.AuthHeader) == "" {
		cfg.MCP.AuthHeader = "X-MCP-Token"
	}
	return persistMCPAuth(path, &cfg.MCP)
}

// English note.
func PrintMCPConfigJSON(mcp MCPConfig) {
	if !mcp.Enabled {
		return
	}
	hostForURL := strings.TrimSpace(mcp.Host)
	if hostForURL == "" || hostForURL == "0.0.0.0" {
		hostForURL = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d/mcp", hostForURL, mcp.Port)
	headers := map[string]string{}
	if mcp.AuthHeader != "" {
		headers[mcp.AuthHeader] = mcp.AuthHeaderValue
	}
	serverEntry := map[string]interface{}{
		"url": url,
	}
	if len(headers) > 0 {
		serverEntry["headers"] = headers
	}
	// English note.
	serverEntry["type"] = "http"
	out := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cyberstrike-ai": serverEntry,
		},
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println("[CyberStrikeAI] MCP configuration for Cursor / Claude Code")
	fmt.Println("  Cursor: add this under mcpServers in ~/.cursor/mcp.json or .cursor/mcp.json")
	fmt.Println("  Claude Code: add this under mcpServers in .mcp.json or ~/.claude.json")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(b))
	fmt.Println("----------------------------------------------------------------")
}

// English note.
func LoadToolsFromDir(dir string) ([]ToolConfig, error) {
	var tools []ToolConfig

	// English note.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return tools, nil // Total퐬띶춼?ⓩ뿶붷썮뷴닓⑨펽띷뒫Total
	}

	// English note.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		tool, err := LoadToolFromFile(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to load tool file %s: %v\n", filePath, err)
			continue
		}

		tools = append(tools, *tool)
	}

	return tools, nil
}

// English note.
func LoadToolFromFile(path string) (*ToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var tool ToolConfig
	if err := yaml.Unmarshal(data, &tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool file: %w", err)
	}

	// English note.
	if tool.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if tool.Command == "" {
		return nil, fmt.Errorf("tool command is required")
	}

	return &tool, nil
}

// English note.
func LoadRolesFromDir(dir string) (map[string]RoleConfig, error) {
	roles := make(map[string]RoleConfig)

	// English note.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return roles, nil // Total퐬띶춼?ⓩ뿶붷썮튾ap뚥툖?ι뵗
	}

	// English note.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read roles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		role, err := LoadRoleFromFile(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to load role file %s: %v\n", filePath, err)
			continue
		}

		// English note.
		roleName := role.Name
		if roleName == "" {
			// English note.
			roleName = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
			role.Name = roleName
		}

		roles[roleName] = *role
	}

	return roles, nil
}

// English note.
func LoadRoleFromFile(path string) (*RoleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var role RoleConfig
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to parse role file: %w", err)
	}

	// English note.
	// English note.
	if role.Icon != "" {
		icon := role.Icon
		// English note.
		icon = strings.Trim(icon, `"`)

		// English note.
		if len(icon) >= 3 && icon[0] == '\\' {
			if icon[1] == 'U' && len(icon) >= 10 {
				// English note.
				if codePoint, err := strconv.ParseInt(icon[2:10], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			} else if icon[1] == 'u' && len(icon) >= 6 {
				// English note.
				if codePoint, err := strconv.ParseInt(icon[2:6], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			}
		}
	}

	// English note.
	if role.Name == "" {
		// English note.
		baseName := filepath.Base(path)
		role.Name = strings.TrimSuffix(strings.TrimSuffix(baseName, ".yaml"), ".yml")
	}

	return &role, nil
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Log: LogConfig{
			Level:  "info",
			Output: "stdout",
		},
		MCP: MCPConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    8081,
		},
		OpenAI: OpenAIConfig{
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4",
			MaxTotalTokens: 120000,
		},
		Agent: AgentConfig{
			MaxIterations:      30, // 섋Total㎬열ｆАTotal
			ToolTimeoutMinutes: 10, // ?뺞Аε끁?㎬죱섋Total?10 ?녽뮓뚪겳?띶펰면빣?띌뿴?좂뵪
		},
		Security: SecurityConfig{
			Tools:    []ToolConfig{}, // ε끁?띸쉰붻??config.yaml Totaltools/ Total퐬?좄슬
			ToolsDir: "tools",        // 섋?ε끁Total퐬
		},
		Database: DatabaseConfig{
			Path:            "data/conversations.db",
			KnowledgeDBPath: "data/knowledge.db", // 섋Totalθ칳볠빊Total틩?푶
		},
		Auth: AuthConfig{
			SessionDurationHours: 12,
		},
		Knowledge: KnowledgeConfig{
			Enabled:  true,
			BasePath: "knowledge_base",
			Embedding: EmbeddingConfig{
				Provider: "openai",
				Model:    "text-embedding-3-small",
				BaseURL:  "https://api.openai.com/v1",
			},
			Retrieval: RetrievalConfig{
				TopK:                5,
				SimilarityThreshold: 0.65, // ?띴퐥?덂쇔댆 0.65뚦뇧묉폀
			},
			Indexing: IndexingConfig{
				ChunkStrategy:         "markdown_then_recursive",
				RequestTimeoutSeconds: 120,
				ChunkSize:             768, // 욃뒥Total768뚧쎍썹쉪듾툔?뉏퓷Total
				ChunkOverlap:          50,
				MaxChunksPerItem:      20, // ?먨댍?뺜릉?θ칳방??20 ゅ쓼뚪겳?띷텋?쀨퓝싮뀓?
				BatchSize:             64,
				PreferSourceFile:      false,
				MaxRPM:                100, // 섋? 100 RPM뚪겳Total429 ?숃?
				RateLimitDelayMs:      600, // 600ms ?닻슂뚦??100 RPM
				MaxRetries:            3,
				RetryDelayMs:          1000,
				SubIndexes:            nil,
			},
		},
	}
}

// English note.
type KnowledgeConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`     // Total맔Total뵪?θ칳?
	BasePath  string          `yaml:"base_path" json:"base_path"` // ?θ칳볢러?
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	Retrieval RetrievalConfig `yaml:"retrieval" json:"retrieval"`
	Indexing  IndexingConfig  `yaml:"indexing,omitempty" json:"indexing,omitempty"` // ℡폊?꾢뻠?띸쉰
}

// English note.
type IndexingConfig struct {
	// English note.
	ChunkStrategy string `yaml:"chunk_strategy,omitempty" json:"chunk_strategy,omitempty"`
	// English note.
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds,omitempty" json:"request_timeout_seconds,omitempty"`
	// English note.
	ChunkSize        int `yaml:"chunk_size,omitempty" json:"chunk_size,omitempty"`                   // 뤶릉?쀧쉪??token ?곤펷곁츞됵펽섋? 512
	ChunkOverlap     int `yaml:"chunk_overlap,omitempty" json:"chunk_overlap,omitempty"`             // ?쀤퉳?당쉪?띶룧 token ?곤펽섋? 50
	MaxChunksPerItem int `yaml:"max_chunks_per_item,omitempty" json:"max_chunks_per_item,omitempty"` // ?뺜릉?θ칳밭쉪?㎩쓼?곈뇧? ①ㅊ띺솏Total

	// English note.
	PreferSourceFile bool `yaml:"prefer_source_file,omitempty" json:"prefer_source_file,omitempty"`

	// English note.
	RateLimitDelayMs int `yaml:"rate_limit_delay_ms,omitempty" json:"rate_limit_delay_ms,omitempty"` // 룡콆?닻슂?띌뿴덃?믭펹? ①ㅊ띴슴?ⓨ쎓싧뻑?
	MaxRPM           int `yaml:"max_rpm,omitempty" json:"max_rpm,omitempty"`                         // 뤷늽?잍?㎬?귝빊? ①ㅊ띺솏Total

	// English note.
	MaxRetries   int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`       // ?㏝뇥뺞А?곤펽섋? 3
	RetryDelayMs int `yaml:"retry_delay_ms,omitempty" json:"retry_delay_ms,omitempty"` // ?띹캊?닻슂덃?믭펹뚪퍡?1000

	// English note.
	BatchSize int `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`
	// English note.
	SubIndexes []string `yaml:"sub_indexes,omitempty" json:"sub_indexes,omitempty"`
}

// English note.
type EmbeddingConfig struct {
	Provider string `yaml:"provider" json:"provider"` // 뚦뀯▼엹?먧풘Total
	Model    string `yaml:"model" json:"model"`       // ▼엹?띸㎞
	BaseURL  string `yaml:"base_url" json:"base_url"` // API Base URL
	APIKey   string `yaml:"api_key" json:"api_key"`   // API Key덁퍗OpenAI?띸쉰㎪돽?
}

// English note.
type PostRetrieveConfig struct {
	// English note.
	PrefetchTopK int `yaml:"prefetch_top_k,omitempty" json:"prefetch_top_k,omitempty"`
	// English note.
	MaxContextChars int `yaml:"max_context_chars,omitempty" json:"max_context_chars,omitempty"`
	// English note.
	MaxContextTokens int `yaml:"max_context_tokens,omitempty" json:"max_context_tokens,omitempty"`
}

// English note.
type RetrievalConfig struct {
	TopK                int     `yaml:"top_k" json:"top_k"`                               // 줥op-K
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"` // 쇿샷?멧세?삁Total
	// English note.
	SubIndexFilter string `yaml:"sub_index_filter,omitempty" json:"sub_index_filter,omitempty"`
	// English note.
	PostRetrieve PostRetrieveConfig `yaml:"post_retrieve,omitempty" json:"post_retrieve,omitempty"`
}

// English note.
// English note.
type RolesConfig struct {
	Roles map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// English note.
type RoleConfig struct {
	Name        string   `yaml:"name" json:"name"`                         // 믦돯?띸㎞
	Description string   `yaml:"description" json:"description"`           // 믦돯?뤺염
	UserPrompt  string   `yaml:"user_prompt" json:"user_prompt"`           // ?ⓩ댎?먪ㅊ?썲뒥?곁뵪?룡텋Total뎺)
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`     // 믦돯?얏젃덂룾?됵펹
	Tools       []string `yaml:"tools,omitempty" json:"tools,omitempty"`   // ?녘걫?꾢램?룟닓⑨펷toolKey?쇔폀뚦쫩 "toolName" Total"mcpName::toolName"?
	MCPs        []string `yaml:"mcps,omitempty" json:"mcps,omitempty"`     // ?묈릮?쇔?싧뀽?붺쉪MCP?띶뒦?ⓨ닓⑨펷꿨틹껓펽욜뵪tools?요빰?
	Skills      []string `yaml:"skills,omitempty" json:"skills,omitempty"` // ?녘걫?꼜kills?쀨〃늮kill?띸㎞?쀨〃뚦쑉?㎬죱삣뒦?띴폏삣룚쇾틳skills?꾢냵뱄펹
	Enabled     bool     `yaml:"enabled" json:"enabled"`                   // Total맔Total뵪
}

