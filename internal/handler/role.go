package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cyberstrike-ai/internal/config"

	"gopkg.in/yaml.v3"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type RoleHandler struct {
	config        *config.Config
	configPath    string
	logger        *zap.Logger
	skillsManager SkillsManager // Skills（）
}

// English note.
type SkillsManager interface {
	ListSkills() ([]string, error)
}

// English note.
func NewRoleHandler(cfg *config.Config, configPath string, logger *zap.Logger) *RoleHandler {
	return &RoleHandler{
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// English note.
func (h *RoleHandler) SetSkillsManager(manager SkillsManager) {
	h.skillsManager = manager
}

// English note.
func (h *RoleHandler) GetSkills(c *gin.Context) {
	if h.skillsManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"skills": []string{},
		})
		return
	}

	skills, err := h.skillsManager.ListSkills()
	if err != nil {
		h.logger.Warn("skills", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{
			"skills": []string{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": skills,
	})
}

// English note.
func (h *RoleHandler) GetRoles(c *gin.Context) {
	if h.config.Roles == nil {
		h.config.Roles = make(map[string]config.RoleConfig)
	}

	roles := make([]config.RoleConfig, 0, len(h.config.Roles))
	for key, role := range h.config.Roles {
		// English note.
		if role.Name == "" {
			role.Name = key
		}
		roles = append(roles, role)
	}

	c.JSON(http.StatusOK, gin.H{
		"roles": roles,
	})
}

// English note.
func (h *RoleHandler) GetRole(c *gin.Context) {
	roleName := c.Param("name")
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	if h.config.Roles == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	role, exists := h.config.Roles[roleName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	// English note.
	if role.Name == "" {
		role.Name = roleName
	}

	c.JSON(http.StatusOK, gin.H{
		"role": role,
	})
}

// English note.
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	roleName := c.Param("name")
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	var req config.RoleConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	// English note.
	if req.Name == "" {
		req.Name = roleName
	}

	// English note.
	if h.config.Roles == nil {
		h.config.Roles = make(map[string]config.RoleConfig)
	}

	// English note.
	// English note.
	finalKey := req.Name
	keysToDelete := make([]string, 0)
	for key := range h.config.Roles {
		// English note.
		if key != finalKey {
			role := h.config.Roles[key]
			// English note.
			if role.Name == "" {
				role.Name = key
			}
			if role.Name == req.Name {
				keysToDelete = append(keysToDelete, key)
			}
		}
	}
	// English note.
	for _, key := range keysToDelete {
		delete(h.config.Roles, key)
		h.logger.Info("", zap.String("oldKey", key), zap.String("name", req.Name))
	}

	// English note.
	if roleName != finalKey {
		delete(h.config.Roles, roleName)
	}

	// English note.
	if roleName != finalKey {
		configDir := filepath.Dir(h.configPath)
		rolesDir := h.config.RolesDir
		if rolesDir == "" {
			rolesDir = "roles" // 
		}

		// English note.
		if !filepath.IsAbs(rolesDir) {
			rolesDir = filepath.Join(configDir, rolesDir)
		}

		// English note.
		oldSafeFileName := sanitizeFileName(roleName)
		oldRoleFileYaml := filepath.Join(rolesDir, oldSafeFileName+".yaml")
		oldRoleFileYml := filepath.Join(rolesDir, oldSafeFileName+".yml")

		if _, err := os.Stat(oldRoleFileYaml); err == nil {
			if err := os.Remove(oldRoleFileYaml); err != nil {
				h.logger.Warn("", zap.String("file", oldRoleFileYaml), zap.Error(err))
			}
		}
		if _, err := os.Stat(oldRoleFileYml); err == nil {
			if err := os.Remove(oldRoleFileYml); err != nil {
				h.logger.Warn("", zap.String("file", oldRoleFileYml), zap.Error(err))
			}
		}
	}

	// English note.
	h.config.Roles[finalKey] = req

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	h.logger.Info("", zap.String("oldKey", roleName), zap.String("newKey", finalKey), zap.String("name", req.Name))
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"role":    req,
	})
}

// English note.
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req config.RoleConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	// English note.
	if h.config.Roles == nil {
		h.config.Roles = make(map[string]config.RoleConfig)
	}

	// English note.
	if _, exists := h.config.Roles[req.Name]; exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	// English note.
	if !req.Enabled {
		req.Enabled = true
	}

	h.config.Roles[req.Name] = req

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	h.logger.Info("", zap.String("roleName", req.Name))
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"role":    req,
	})
}

// English note.
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	roleName := c.Param("name")
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	if h.config.Roles == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	if _, exists := h.config.Roles[roleName]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	// English note.
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	delete(h.config.Roles, roleName)

	// English note.
	configDir := filepath.Dir(h.configPath)
	rolesDir := h.config.RolesDir
	if rolesDir == "" {
		rolesDir = "roles" // 
	}

	// English note.
	if !filepath.IsAbs(rolesDir) {
		rolesDir = filepath.Join(configDir, rolesDir)
	}

	// English note.
	safeFileName := sanitizeFileName(roleName)
	roleFileYaml := filepath.Join(rolesDir, safeFileName+".yaml")
	roleFileYml := filepath.Join(rolesDir, safeFileName+".yml")

	// English note.
	if _, err := os.Stat(roleFileYaml); err == nil {
		if err := os.Remove(roleFileYaml); err != nil {
			h.logger.Warn("", zap.String("file", roleFileYaml), zap.Error(err))
		} else {
			h.logger.Info("", zap.String("file", roleFileYaml))
		}
	}

	// English note.
	if _, err := os.Stat(roleFileYml); err == nil {
		if err := os.Remove(roleFileYml); err != nil {
			h.logger.Warn("", zap.String("file", roleFileYml), zap.Error(err))
		} else {
			h.logger.Info("", zap.String("file", roleFileYml))
		}
	}

	h.logger.Info("", zap.String("roleName", roleName))
	c.JSON(http.StatusOK, gin.H{
		"message": "",
	})
}

// English note.
func (h *RoleHandler) saveConfig() error {
	configDir := filepath.Dir(h.configPath)
	rolesDir := h.config.RolesDir
	if rolesDir == "" {
		rolesDir = "roles" // 
	}

	// English note.
	if !filepath.IsAbs(rolesDir) {
		rolesDir = filepath.Join(configDir, rolesDir)
	}

	// English note.
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	if h.config.Roles != nil {
		for roleName, role := range h.config.Roles {
			// English note.
			if role.Name == "" {
				role.Name = roleName
			}

			// English note.
			safeFileName := sanitizeFileName(role.Name)
			roleFile := filepath.Join(rolesDir, safeFileName+".yaml")

			// English note.
			roleData, err := yaml.Marshal(&role)
			if err != nil {
				h.logger.Error("", zap.String("role", roleName), zap.Error(err))
				continue
			}

			// English note.
			roleDataStr := string(roleData)
			if role.Icon != "" && strings.HasPrefix(role.Icon, "\\U") {
				// English note.
				// English note.
				re := regexp.MustCompile(`(?m)^(icon:\s+)(\\U[0-9A-F]{8})(\s*)$`)
				roleDataStr = re.ReplaceAllString(roleDataStr, `${1}"${2}"${3}`)
				roleData = []byte(roleDataStr)
			}

			// English note.
			if err := os.WriteFile(roleFile, roleData, 0644); err != nil {
				h.logger.Error("", zap.String("role", roleName), zap.String("file", roleFile), zap.Error(err))
				continue
			}

			h.logger.Info("", zap.String("role", roleName), zap.String("file", roleFile))
		}
	}

	return nil
}

// English note.
func sanitizeFileName(name string) string {
	// English note.
	replacer := map[rune]string{
		'/':  "_",
		'\\': "_",
		':':  "_",
		'*':  "_",
		'?':  "_",
		'"':  "_",
		'<':  "_",
		'>':  "_",
		'|':  "_",
		' ':  "_",
	}

	var result []rune
	for _, r := range name {
		if replacement, ok := replacer[r]; ok {
			result = append(result, []rune(replacement)...)
		} else {
			result = append(result, r)
		}
	}

	fileName := string(result)
	// English note.
	if fileName == "" {
		fileName = "role"
	}

	return fileName
}

// English note.
func updateRolesConfig(doc *yaml.Node, cfg config.RolesConfig) {
	root := doc.Content[0]
	rolesNode := ensureMap(root, "roles")

	// English note.
	if rolesNode.Kind == yaml.MappingNode {
		rolesNode.Content = nil
	}

	// English note.
	if cfg.Roles != nil {
		// English note.
		rolesByName := make(map[string]config.RoleConfig)
		for roleKey, role := range cfg.Roles {
			// English note.
			if role.Name == "" {
				role.Name = roleKey
			}
			// English note.
			rolesByName[role.Name] = role
		}

		// English note.
		for roleName, role := range rolesByName {
			roleNode := ensureMap(rolesNode, roleName)
			setStringInMap(roleNode, "name", role.Name)
			setStringInMap(roleNode, "description", role.Description)
			setStringInMap(roleNode, "user_prompt", role.UserPrompt)
			if role.Icon != "" {
				setStringInMap(roleNode, "icon", role.Icon)
			}
			setBoolInMap(roleNode, "enabled", role.Enabled)

			// English note.
			if len(role.Tools) > 0 {
				toolsNode := ensureArray(roleNode, "tools")
				toolsNode.Content = nil
				for _, toolKey := range role.Tools {
					toolNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: toolKey}
					toolsNode.Content = append(toolsNode.Content, toolNode)
				}
			} else if len(role.MCPs) > 0 {
				// English note.
				mcpsNode := ensureArray(roleNode, "mcps")
				mcpsNode.Content = nil
				for _, mcpName := range role.MCPs {
					mcpNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mcpName}
					mcpsNode.Content = append(mcpsNode.Content, mcpNode)
				}
			}
		}
	}
}

// English note.
func ensureArray(parent *yaml.Node, key string) *yaml.Node {
	_, valueNode := ensureKeyValue(parent, key)
	if valueNode.Kind != yaml.SequenceNode {
		valueNode.Kind = yaml.SequenceNode
		valueNode.Tag = "!!seq"
		valueNode.Content = nil
	}
	return valueNode
}
