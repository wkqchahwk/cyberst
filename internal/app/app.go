package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/handler"
	"cyberstrike-ai/internal/knowledge"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/robot"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/skillpackage"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// English note.
type App struct {
	config             *config.Config
	logger             *logger.Logger
	router             *gin.Engine
	mcpServer          *mcp.Server
	externalMCPMgr     *mcp.ExternalMCPManager
	agent              *agent.Agent
	executor           *security.Executor
	db                 *database.DB
	knowledgeDB        *database.DB // （）
	auth               *security.AuthManager
	knowledgeManager   *knowledge.Manager        // （）
	knowledgeRetriever *knowledge.Retriever      // （）
	knowledgeIndexer   *knowledge.Indexer        // （）
	knowledgeHandler   *handler.KnowledgeHandler // （）
	agentHandler       *handler.AgentHandler     // Agent（）
	robotHandler       *handler.RobotHandler     // （//）
	robotMu            sync.Mutex                // / cancel
	dingCancel         context.CancelFunc        //  Stream ，
	larkCancel         context.CancelFunc        // ，
}

// English note.
func New(cfg *config.Config, log *logger.Logger) (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// English note.
	router.Use(corsMiddleware())

	// English note.
	authManager, err := security.NewAuthManager(cfg.Auth.Password, cfg.Auth.SessionDurationHours)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data/conversations.db"
	}

	// English note.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	db, err := database.NewDB(dbPath, log.Logger)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	mcpServer := mcp.NewServerWithStorage(log.Logger, db)

	// English note.
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// English note.
	executor.RegisterTools(mcpServer)

	// English note.
	registerVulnerabilityTool(mcpServer, db, log.Logger)

	if cfg.Auth.GeneratedPassword != "" {
		config.PrintGeneratedPasswordWarning(cfg.Auth.GeneratedPassword, cfg.Auth.GeneratedPasswordPersisted, cfg.Auth.GeneratedPasswordPersistErr)
		cfg.Auth.GeneratedPassword = ""
		cfg.Auth.GeneratedPasswordPersisted = false
		cfg.Auth.GeneratedPasswordPersistErr = ""
	}

	// English note.
	externalMCPMgr := mcp.NewExternalMCPManagerWithStorage(log.Logger, db)
	if cfg.ExternalMCP.Servers != nil {
		externalMCPMgr.LoadConfigs(&cfg.ExternalMCP)
		// English note.
		externalMCPMgr.StartAllEnabled()
	}

	// English note.
	resultStorageDir := "tmp"
	if cfg.Agent.ResultStorageDir != "" {
		resultStorageDir = cfg.Agent.ResultStorageDir
	}

	// English note.
	if err := os.MkdirAll(resultStorageDir, 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	resultStorage, err := storage.NewFileResultStorage(resultStorageDir, log.Logger)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	maxIterations := cfg.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30 // 
	}
	agent := agent.NewAgent(&cfg.OpenAI, &cfg.Agent, mcpServer, externalMCPMgr, log.Logger, maxIterations)

	// English note.
	agent.SetResultStorage(resultStorage)

	// English note.
	executor.SetResultStorage(resultStorage)

	// English note.
	var knowledgeManager *knowledge.Manager
	var knowledgeRetriever *knowledge.Retriever
	var knowledgeIndexer *knowledge.Indexer
	var knowledgeHandler *handler.KnowledgeHandler

	var knowledgeDBConn *database.DB
	log.Logger.Info("", zap.Bool("enabled", cfg.Knowledge.Enabled))
	if cfg.Knowledge.Enabled {
		// English note.
		knowledgeDBPath := cfg.Database.KnowledgeDBPath
		var knowledgeDB *sql.DB

		if knowledgeDBPath != "" {
			// English note.
			// English note.
			if err := os.MkdirAll(filepath.Dir(knowledgeDBPath), 0755); err != nil {
				return nil, fmt.Errorf(": %w", err)
			}

			var err error
			knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, log.Logger)
			if err != nil {
				return nil, fmt.Errorf(": %w", err)
			}
			knowledgeDB = knowledgeDBConn.DB
			log.Logger.Info("", zap.String("path", knowledgeDBPath))
		} else {
			// English note.
			knowledgeDB = db.DB
			log.Logger.Info("（knowledge_db_path）")
		}

		// English note.
		knowledgeManager = knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, log.Logger)

		// English note.
		// English note.
		if cfg.Knowledge.Embedding.APIKey == "" {
			cfg.Knowledge.Embedding.APIKey = cfg.OpenAI.APIKey
		}
		if cfg.Knowledge.Embedding.BaseURL == "" {
			cfg.Knowledge.Embedding.BaseURL = cfg.OpenAI.BaseURL
		}

		embedder, err := knowledge.NewEmbedder(context.Background(), &cfg.Knowledge, &cfg.OpenAI, log.Logger)
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                cfg.Knowledge.Retrieval.TopK,
			SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
			SubIndexFilter:      cfg.Knowledge.Retrieval.SubIndexFilter,
			PostRetrieve:        cfg.Knowledge.Retrieval.PostRetrieve,
		}
		knowledgeRetriever = knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, log.Logger)

		// English note.
		knowledgeIndexer, err = knowledge.NewIndexer(context.Background(), knowledgeDB, embedder, log.Logger, &cfg.Knowledge)
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)

		// English note.
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, log.Logger)
		log.Logger.Info("", zap.Bool("handler_created", knowledgeHandler != nil))

		// English note.
		go func() {
			itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
			if err != nil {
				log.Logger.Warn("", zap.Error(err))
				return
			}

			// English note.
			hasIndex, err := knowledgeIndexer.HasIndex()
			if err != nil {
				log.Logger.Warn("", zap.Error(err))
				return
			}

			if hasIndex {
				// English note.
				if len(itemsToIndex) > 0 {
					log.Logger.Info("，", zap.Int("count", len(itemsToIndex)))
					ctx := context.Background()
					consecutiveFailures := 0
					var firstFailureItemID string
					var firstFailureError error
					failedCount := 0

					for _, itemID := range itemsToIndex {
						if err := knowledgeIndexer.IndexItem(ctx, itemID); err != nil {
							failedCount++
							consecutiveFailures++

							if consecutiveFailures == 1 {
								firstFailureItemID = itemID
								firstFailureError = err
								log.Logger.Warn("", zap.String("itemId", itemID), zap.Error(err))
							}

							// English note.
							if consecutiveFailures >= 2 {
								log.Logger.Error("，",
									zap.Int("consecutiveFailures", consecutiveFailures),
									zap.Int("totalItems", len(itemsToIndex)),
									zap.String("firstFailureItemId", firstFailureItemID),
									zap.Error(firstFailureError),
								)
								break
							}
							continue
						}

						// English note.
						if consecutiveFailures > 0 {
							consecutiveFailures = 0
							firstFailureItemID = ""
							firstFailureError = nil
						}
					}
					log.Logger.Info("", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
				} else {
					log.Logger.Info("，")
				}
				return
			}

			// English note.
			log.Logger.Info("，")
			ctx := context.Background()
			if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
				log.Logger.Warn("", zap.Error(err))
			}
		}()
	}

	// English note.
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	skillsDir := skillpackage.SkillsRootFromConfig(cfg.SkillsDir, configPath)
	log.Logger.Info("Skills （Eino ADK skill  + Web  API）", zap.String("skillsDir", skillsDir))
	configDir := filepath.Dir(configPath)
	agent.SetPromptBaseDir(configDir)

	agentsDir := cfg.AgentsDir
	if agentsDir == "" {
		agentsDir = "agents"
	}
	if !filepath.IsAbs(agentsDir) {
		agentsDir = filepath.Join(configDir, agentsDir)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		log.Logger.Warn(" agents ", zap.String("path", agentsDir), zap.Error(err))
	}
	markdownAgentsHandler := handler.NewMarkdownAgentsHandler(agentsDir)
	log.Logger.Info(" Markdown  Agent ", zap.String("agentsDir", agentsDir))

	// English note.
	agentHandler := handler.NewAgentHandler(agent, db, cfg, log.Logger)
	agentHandler.SetAgentsMarkdownDir(agentsDir)
	// English note.
	if knowledgeManager != nil {
		agentHandler.SetKnowledgeManager(knowledgeManager)
	}
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetExternalMCPManager(externalMCPMgr) // MCP，MCP
	groupHandler := handler.NewGroupHandler(db, log.Logger)
	authHandler := handler.NewAuthHandler(authManager, cfg, configPath, log.Logger)
	attackChainHandler := handler.NewAttackChainHandler(db, &cfg.OpenAI, log.Logger)
	vulnerabilityHandler := handler.NewVulnerabilityHandler(db, log.Logger)
	webshellHandler := handler.NewWebShellHandler(log.Logger, db)
	chatUploadsHandler := handler.NewChatUploadsHandler(log.Logger)
	registerWebshellTools(mcpServer, db, webshellHandler, log.Logger)
	registerWebshellManagementTools(mcpServer, db, webshellHandler, log.Logger)
	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, agent, attackChainHandler, externalMCPMgr, log.Logger)
	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)
	roleHandler := handler.NewRoleHandler(cfg, configPath, log.Logger)
	roleHandler.SetSkillsManager(skillpackage.DirLister{SkillsRoot: skillsDir})
	skillsHandler := handler.NewSkillsHandler(cfg, configPath, log.Logger)
	fofaHandler := handler.NewFofaHandler(cfg, log.Logger)
	terminalHandler := handler.NewTerminalHandler(log.Logger)
	if db != nil {
		skillsHandler.SetDB(db) // 
	}

	// English note.
	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	robotHandler := handler.NewRobotHandler(cfg, db, agentHandler, log.Logger)
	openAPIHandler := handler.NewOpenAPIHandler(db, log.Logger, resultStorage, conversationHandler, agentHandler)

	// English note.
	app := &App{
		config:             cfg,
		logger:             log,
		router:             router,
		mcpServer:          mcpServer,
		externalMCPMgr:     externalMCPMgr,
		agent:              agent,
		executor:           executor,
		db:                 db,
		knowledgeDB:        knowledgeDBConn,
		auth:               authManager,
		knowledgeManager:   knowledgeManager,
		knowledgeRetriever: knowledgeRetriever,
		knowledgeIndexer:   knowledgeIndexer,
		knowledgeHandler:   knowledgeHandler,
		agentHandler:       agentHandler,
		robotHandler:       robotHandler,
	}
	// English note.
	app.startRobotConnections()

	// English note.
	vulnerabilityRegistrar := func() error {
		registerVulnerabilityTool(mcpServer, db, log.Logger)
		return nil
	}
	configHandler.SetVulnerabilityToolRegistrar(vulnerabilityRegistrar)

	// English note.
	webshellRegistrar := func() error {
		registerWebshellTools(mcpServer, db, webshellHandler, log.Logger)
		registerWebshellManagementTools(mcpServer, db, webshellHandler, log.Logger)
		return nil
	}
	configHandler.SetWebshellToolRegistrar(webshellRegistrar)

	// English note.
	configHandler.SetSkillsToolRegistrar(func() error { return nil })

	handler.RegisterBatchTaskMCPTools(mcpServer, agentHandler, log.Logger)
	batchTaskToolRegistrar := func() error {
		handler.RegisterBatchTaskMCPTools(mcpServer, agentHandler, log.Logger)
		return nil
	}
	configHandler.SetBatchTaskToolRegistrar(batchTaskToolRegistrar)

	// English note.
	configHandler.SetKnowledgeInitializer(func() (*handler.KnowledgeHandler, error) {
		knowledgeHandler, err := initializeKnowledge(cfg, db, knowledgeDBConn, mcpServer, agentHandler, app, log.Logger)
		if err != nil {
			return nil, err
		}

		// English note.
		// English note.
		if app.knowledgeRetriever != nil && app.knowledgeManager != nil {
			// English note.
			registrar := func() error {
				knowledge.RegisterKnowledgeTool(mcpServer, app.knowledgeRetriever, app.knowledgeManager, log.Logger)
				return nil
			}
			configHandler.SetKnowledgeToolRegistrar(registrar)
			// English note.
			configHandler.SetRetrieverUpdater(app.knowledgeRetriever)
			log.Logger.Info("")
		}

		return knowledgeHandler, nil
	})

	// English note.
	if cfg.Knowledge.Enabled && knowledgeRetriever != nil && knowledgeManager != nil {
		// English note.
		registrar := func() error {
			knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)
			return nil
		}
		configHandler.SetKnowledgeToolRegistrar(registrar)
		// English note.
		configHandler.SetRetrieverUpdater(knowledgeRetriever)
	}

	// English note.
	configHandler.SetRobotRestarter(app)

	// English note.
	setupRoutes(
		router,
		authHandler,
		agentHandler,
		monitorHandler,
		conversationHandler,
		robotHandler,
		groupHandler,
		configHandler,
		externalMCPHandler,
		attackChainHandler,
		app, //  App  knowledgeHandler
		vulnerabilityHandler,
		webshellHandler,
		chatUploadsHandler,
		roleHandler,
		skillsHandler,
		markdownAgentsHandler,
		fofaHandler,
		terminalHandler,
		mcpServer,
		authManager,
		openAPIHandler,
	)

	return app, nil

}

// English note.
func (a *App) mcpHandlerWithAuth(w http.ResponseWriter, r *http.Request) {
	cfg := a.config.MCP
	if cfg.AuthHeader != "" {
		if r.Header.Get(cfg.AuthHeader) != cfg.AuthHeaderValue {
			a.logger.Logger.Debug("MCP ：header ", zap.String("header", cfg.AuthHeader))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
	}
	a.mcpServer.HandleHTTP(w, r)
}

// English note.
func (a *App) Run() error {
	// English note.
	if a.config.MCP.Enabled {
		go func() {
			mcpAddr := fmt.Sprintf("%s:%d", a.config.MCP.Host, a.config.MCP.Port)
			a.logger.Info("MCP", zap.String("address", mcpAddr))

			mux := http.NewServeMux()
			mux.HandleFunc("/mcp", a.mcpHandlerWithAuth)

			if err := http.ListenAndServe(mcpAddr, mux); err != nil {
				a.logger.Error("MCP", zap.Error(err))
			}
		}()
	}

	// English note.
	addr := fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port)
	a.logger.Info("HTTP", zap.String("address", addr))

	return a.router.Run(addr)
}

// English note.
func (a *App) Shutdown() {
	// English note.
	a.robotMu.Lock()
	if a.dingCancel != nil {
		a.dingCancel()
		a.dingCancel = nil
	}
	if a.larkCancel != nil {
		a.larkCancel()
		a.larkCancel = nil
	}
	a.robotMu.Unlock()

	// English note.
	if a.externalMCPMgr != nil {
		a.externalMCPMgr.StopAll()
	}

	// English note.
	if a.knowledgeDB != nil {
		if err := a.knowledgeDB.Close(); err != nil {
			a.logger.Logger.Warn("", zap.Error(err))
		}
	}
}

// English note.
func (a *App) startRobotConnections() {
	a.robotMu.Lock()
	defer a.robotMu.Unlock()
	cfg := a.config
	if cfg.Robots.Lark.Enabled && cfg.Robots.Lark.AppID != "" && cfg.Robots.Lark.AppSecret != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.larkCancel = cancel
		go robot.StartLark(ctx, cfg.Robots.Lark, a.robotHandler, a.logger.Logger)
	}
	if cfg.Robots.Dingtalk.Enabled && cfg.Robots.Dingtalk.ClientID != "" && cfg.Robots.Dingtalk.ClientSecret != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.dingCancel = cancel
		go robot.StartDing(ctx, cfg.Robots.Dingtalk, a.robotHandler, a.logger.Logger)
	}
}

// English note.
func (a *App) RestartRobotConnections() {
	a.robotMu.Lock()
	if a.dingCancel != nil {
		a.dingCancel()
		a.dingCancel = nil
	}
	if a.larkCancel != nil {
		a.larkCancel()
		a.larkCancel = nil
	}
	a.robotMu.Unlock()
	// English note.
	time.Sleep(200 * time.Millisecond)
	a.startRobotConnections()
}

// English note.
func setupRoutes(
	router *gin.Engine,
	authHandler *handler.AuthHandler,
	agentHandler *handler.AgentHandler,
	monitorHandler *handler.MonitorHandler,
	conversationHandler *handler.ConversationHandler,
	robotHandler *handler.RobotHandler,
	groupHandler *handler.GroupHandler,
	configHandler *handler.ConfigHandler,
	externalMCPHandler *handler.ExternalMCPHandler,
	attackChainHandler *handler.AttackChainHandler,
	app *App, //  App  knowledgeHandler
	vulnerabilityHandler *handler.VulnerabilityHandler,
	webshellHandler *handler.WebShellHandler,
	chatUploadsHandler *handler.ChatUploadsHandler,
	roleHandler *handler.RoleHandler,
	skillsHandler *handler.SkillsHandler,
	markdownAgentsHandler *handler.MarkdownAgentsHandler,
	fofaHandler *handler.FofaHandler,
	terminalHandler *handler.TerminalHandler,
	mcpServer *mcp.Server,
	authManager *security.AuthManager,
	openAPIHandler *handler.OpenAPIHandler,
) {
	// English note.
	api := router.Group("/api")

	// English note.
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/logout", security.AuthMiddleware(authManager), authHandler.Logout)
		authRoutes.POST("/change-password", security.AuthMiddleware(authManager), authHandler.ChangePassword)
		authRoutes.GET("/validate", security.AuthMiddleware(authManager), authHandler.Validate)
	}

	// English note.
	api.GET("/robot/wecom", robotHandler.HandleWecomGET)
	api.POST("/robot/wecom", robotHandler.HandleWecomPOST)
	api.POST("/robot/dingtalk", robotHandler.HandleDingtalkPOST)
	api.POST("/robot/lark", robotHandler.HandleLarkPOST)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	{
		// English note.
		protected.POST("/robot/test", robotHandler.HandleRobotTest)

		// Agent Loop
		protected.POST("/agent-loop", agentHandler.AgentLoop)
		// English note.
		protected.POST("/agent-loop/stream", agentHandler.AgentLoopStream)
		// English note.
		protected.POST("/eino-agent", agentHandler.EinoSingleAgentLoop)
		protected.POST("/eino-agent/stream", agentHandler.EinoSingleAgentLoopStream)
		// English note.
		protected.POST("/agent-loop/cancel", agentHandler.CancelAgentLoop)
		protected.GET("/agent-loop/tasks", agentHandler.ListAgentTasks)
		protected.GET("/agent-loop/tasks/completed", agentHandler.ListCompletedTasks)

		// English note.
		// English note.
		protected.POST("/multi-agent", agentHandler.MultiAgentLoop)
		protected.POST("/multi-agent/stream", agentHandler.MultiAgentLoopStream)
		protected.GET("/multi-agent/markdown-agents", markdownAgentsHandler.ListMarkdownAgents)
		protected.GET("/multi-agent/markdown-agents/:filename", markdownAgentsHandler.GetMarkdownAgent)
		protected.POST("/multi-agent/markdown-agents", markdownAgentsHandler.CreateMarkdownAgent)
		protected.PUT("/multi-agent/markdown-agents/:filename", markdownAgentsHandler.UpdateMarkdownAgent)
		protected.DELETE("/multi-agent/markdown-agents/:filename", markdownAgentsHandler.DeleteMarkdownAgent)

		// English note.
		protected.POST("/fofa/search", fofaHandler.Search)
		// English note.
		protected.POST("/fofa/parse", fofaHandler.ParseNaturalLanguage)

		// English note.
		protected.POST("/batch-tasks", agentHandler.CreateBatchQueue)
		protected.GET("/batch-tasks", agentHandler.ListBatchQueues)
		protected.GET("/batch-tasks/:queueId", agentHandler.GetBatchQueue)
		protected.POST("/batch-tasks/:queueId/start", agentHandler.StartBatchQueue)
		protected.POST("/batch-tasks/:queueId/rerun", agentHandler.RerunBatchQueue)
		protected.POST("/batch-tasks/:queueId/pause", agentHandler.PauseBatchQueue)
		protected.PUT("/batch-tasks/:queueId/metadata", agentHandler.UpdateBatchQueueMetadata)
		protected.PUT("/batch-tasks/:queueId/schedule", agentHandler.UpdateBatchQueueSchedule)
		protected.PUT("/batch-tasks/:queueId/schedule-enabled", agentHandler.SetBatchQueueScheduleEnabled)
		protected.DELETE("/batch-tasks/:queueId", agentHandler.DeleteBatchQueue)
		protected.PUT("/batch-tasks/:queueId/tasks/:taskId", agentHandler.UpdateBatchTask)
		protected.POST("/batch-tasks/:queueId/tasks", agentHandler.AddBatchTask)
		protected.DELETE("/batch-tasks/:queueId/tasks/:taskId", agentHandler.DeleteBatchTask)

		// English note.
		protected.POST("/conversations", conversationHandler.CreateConversation)
		protected.GET("/conversations", conversationHandler.ListConversations)
		protected.GET("/conversations/:id", conversationHandler.GetConversation)
		protected.GET("/messages/:id/process-details", conversationHandler.GetMessageProcessDetails)
		protected.PUT("/conversations/:id", conversationHandler.UpdateConversation)
		protected.DELETE("/conversations/:id", conversationHandler.DeleteConversation)
		protected.POST("/conversations/:id/delete-turn", conversationHandler.DeleteConversationTurn)
		protected.PUT("/conversations/:id/pinned", groupHandler.UpdateConversationPinned)

		// English note.
		protected.POST("/groups", groupHandler.CreateGroup)
		protected.GET("/groups", groupHandler.ListGroups)
		protected.GET("/groups/:id", groupHandler.GetGroup)
		protected.PUT("/groups/:id", groupHandler.UpdateGroup)
		protected.DELETE("/groups/:id", groupHandler.DeleteGroup)
		protected.PUT("/groups/:id/pinned", groupHandler.UpdateGroupPinned)
		protected.GET("/groups/:id/conversations", groupHandler.GetGroupConversations)
		protected.GET("/groups/mappings", groupHandler.GetAllMappings)
		protected.POST("/groups/conversations", groupHandler.AddConversationToGroup)
		protected.DELETE("/groups/:id/conversations/:conversationId", groupHandler.RemoveConversationFromGroup)
		protected.PUT("/groups/:id/conversations/:conversationId/pinned", groupHandler.UpdateConversationPinnedInGroup)

		// English note.
		protected.GET("/monitor", monitorHandler.Monitor)
		protected.GET("/monitor/execution/:id", monitorHandler.GetExecution)
		protected.POST("/monitor/executions/names", monitorHandler.BatchGetToolNames)
		protected.DELETE("/monitor/execution/:id", monitorHandler.DeleteExecution)
		protected.DELETE("/monitor/executions", monitorHandler.DeleteExecutions)
		protected.GET("/monitor/stats", monitorHandler.GetStats)

		// English note.
		protected.GET("/config", configHandler.GetConfig)
		protected.GET("/config/tools", configHandler.GetTools)
		protected.PUT("/config", configHandler.UpdateConfig)
		protected.POST("/config/apply", configHandler.ApplyConfig)
		protected.POST("/config/test-openai", configHandler.TestOpenAI)

		// English note.
		protected.POST("/terminal/run", terminalHandler.RunCommand)
		protected.POST("/terminal/run/stream", terminalHandler.RunCommandStream)
		protected.GET("/terminal/ws", terminalHandler.RunCommandWS)

		// English note.
		protected.GET("/external-mcp", externalMCPHandler.GetExternalMCPs)
		protected.GET("/external-mcp/stats", externalMCPHandler.GetExternalMCPStats)
		protected.GET("/external-mcp/:name", externalMCPHandler.GetExternalMCP)
		protected.PUT("/external-mcp/:name", externalMCPHandler.AddOrUpdateExternalMCP)
		protected.DELETE("/external-mcp/:name", externalMCPHandler.DeleteExternalMCP)
		protected.POST("/external-mcp/:name/start", externalMCPHandler.StartExternalMCP)
		protected.POST("/external-mcp/:name/stop", externalMCPHandler.StopExternalMCP)

		// English note.
		protected.GET("/attack-chain/:conversationId", attackChainHandler.GetAttackChain)
		protected.POST("/attack-chain/:conversationId/regenerate", attackChainHandler.RegenerateAttackChain)

		// English note.
		knowledgeRoutes := protected.Group("/knowledge")
		{
			knowledgeRoutes.GET("/categories", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"categories": []string{},
						"enabled":    false,
						"message":    "，",
					})
					return
				}
				app.knowledgeHandler.GetCategories(c)
			})
			knowledgeRoutes.GET("/items", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"items":   []interface{}{},
						"enabled": false,
						"message": "，",
					})
					return
				}
				app.knowledgeHandler.GetItems(c)
			})
			knowledgeRoutes.GET("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"message": "，",
					})
					return
				}
				app.knowledgeHandler.GetItem(c)
			})
			knowledgeRoutes.POST("/items", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.CreateItem(c)
			})
			knowledgeRoutes.PUT("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.UpdateItem(c)
			})
			knowledgeRoutes.DELETE("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.DeleteItem(c)
			})
			knowledgeRoutes.GET("/index-status", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_items":      0,
						"indexed_items":    0,
						"progress_percent": 0,
						"is_complete":      false,
						"message":          "，",
					})
					return
				}
				app.knowledgeHandler.GetIndexStatus(c)
			})
			knowledgeRoutes.POST("/index", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.RebuildIndex(c)
			})
			knowledgeRoutes.POST("/scan", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.ScanKnowledgeBase(c)
			})
			knowledgeRoutes.GET("/retrieval-logs", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"logs":    []interface{}{},
						"enabled": false,
						"message": "，",
					})
					return
				}
				app.knowledgeHandler.GetRetrievalLogs(c)
			})
			knowledgeRoutes.DELETE("/retrieval-logs/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "，",
					})
					return
				}
				app.knowledgeHandler.DeleteRetrievalLog(c)
			})
			knowledgeRoutes.POST("/search", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"results": []interface{}{},
						"enabled": false,
						"message": "，",
					})
					return
				}
				app.knowledgeHandler.Search(c)
			})
			knowledgeRoutes.GET("/stats", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_categories": 0,
						"total_items":      0,
						"message":          "，",
					})
					return
				}
				app.knowledgeHandler.GetStats(c)
			})
		}

		// English note.
		protected.GET("/vulnerabilities", vulnerabilityHandler.ListVulnerabilities)
		protected.GET("/vulnerabilities/stats", vulnerabilityHandler.GetVulnerabilityStats)
		protected.GET("/vulnerabilities/:id", vulnerabilityHandler.GetVulnerability)
		protected.POST("/vulnerabilities", vulnerabilityHandler.CreateVulnerability)
		protected.PUT("/vulnerabilities/:id", vulnerabilityHandler.UpdateVulnerability)
		protected.DELETE("/vulnerabilities/:id", vulnerabilityHandler.DeleteVulnerability)

		// English note.
		protected.GET("/webshell/connections", webshellHandler.ListConnections)
		protected.POST("/webshell/connections", webshellHandler.CreateConnection)
		protected.GET("/webshell/connections/:id/ai-history", webshellHandler.GetAIHistory)
		protected.GET("/webshell/connections/:id/ai-conversations", webshellHandler.ListAIConversations)
		protected.GET("/webshell/connections/:id/state", webshellHandler.GetConnectionState)
		protected.PUT("/webshell/connections/:id", webshellHandler.UpdateConnection)
		protected.PUT("/webshell/connections/:id/state", webshellHandler.SaveConnectionState)
		protected.DELETE("/webshell/connections/:id", webshellHandler.DeleteConnection)
		protected.POST("/webshell/exec", webshellHandler.Exec)
		protected.POST("/webshell/file", webshellHandler.FileOp)

		// English note.
		protected.GET("/chat-uploads", chatUploadsHandler.List)
		protected.GET("/chat-uploads/download", chatUploadsHandler.Download)
		protected.GET("/chat-uploads/content", chatUploadsHandler.GetContent)
		protected.POST("/chat-uploads", chatUploadsHandler.Upload)
		protected.POST("/chat-uploads/mkdir", chatUploadsHandler.Mkdir)
		protected.DELETE("/chat-uploads", chatUploadsHandler.Delete)
		protected.PUT("/chat-uploads/rename", chatUploadsHandler.Rename)
		protected.PUT("/chat-uploads/content", chatUploadsHandler.PutContent)

		// English note.
		protected.GET("/roles", roleHandler.GetRoles)
		protected.GET("/roles/:name", roleHandler.GetRole)
		protected.GET("/roles/skills/list", roleHandler.GetSkills)
		protected.POST("/roles", roleHandler.CreateRole)
		protected.PUT("/roles/:name", roleHandler.UpdateRole)
		protected.DELETE("/roles/:name", roleHandler.DeleteRole)

		// English note.
		protected.GET("/skills", skillsHandler.GetSkills)
		protected.GET("/skills/stats", skillsHandler.GetSkillStats)
		protected.DELETE("/skills/stats", skillsHandler.ClearSkillStats)
		protected.GET("/skills/:name/files", skillsHandler.ListSkillPackageFiles)
		protected.GET("/skills/:name/file", skillsHandler.GetSkillPackageFile)
		protected.PUT("/skills/:name/file", skillsHandler.PutSkillPackageFile)
		protected.PUT("/skills/:name/enabled", skillsHandler.SetSkillEnabled)
		protected.GET("/skills/:name/bound-roles", skillsHandler.GetSkillBoundRoles)
		protected.POST("/skills", skillsHandler.CreateSkill)
		protected.PUT("/skills/:name", skillsHandler.UpdateSkill)
		protected.DELETE("/skills/:name", skillsHandler.DeleteSkill)
		protected.DELETE("/skills/:name/stats", skillsHandler.ClearSkillStatsByName)
		protected.GET("/skills/:name", skillsHandler.GetSkill)

		// English note.
		protected.POST("/mcp", func(c *gin.Context) {
			mcpServer.HandleHTTP(c.Writer, c.Request)
		})

		// English note.
		protected.GET("/conversations/:id/results", openAPIHandler.GetConversationResults)
	}

	// English note.
	protected.GET("/openapi/spec", openAPIHandler.GetOpenAPISpec)

	// English note.
	router.GET("/api-docs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "api-docs.html", nil)
	})

	// English note.
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// English note.
	router.GET("/", func(c *gin.Context) {
		version := app.config.Version
		if version == "" {
			version = "v1.0.0"
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"Version": version})
	})
}

// English note.
func registerVulnerabilityTool(mcpServer *mcp.Server, db *database.DB, logger *zap.Logger) {
	tool := mcp.Tool{
		Name:             builtin.ToolRecordVulnerability,
		Description:      "。，，、、、、、、。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "（）",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "：critical（）、high（）、medium（）、low（）、info（）",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
				},
				"vulnerability_type": map[string]interface{}{
					"type":        "string",
					"description": "，：SQL、XSS、CSRF、",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "（URL、IP、）",
				},
				"proof": map[string]interface{}{
					"type":        "string",
					"description": "（POC、、/）",
				},
				"impact": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
				"recommendation": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
			},
			"required": []string{"title", "severity"},
		},
	}

	handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		// English note.
		conversationID, _ := args["conversation_id"].(string)
		if conversationID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: ": conversation_id 。，。",
					},
				},
				IsError: true,
			}, nil
		}

		title, ok := args["title"].(string)
		if !ok || title == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: ": title ",
					},
				},
				IsError: true,
			}, nil
		}

		severity, ok := args["severity"].(string)
		if !ok || severity == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: ": severity ",
					},
				},
				IsError: true,
			}, nil
		}

		// English note.
		validSeverities := map[string]bool{
			"critical": true,
			"high":     true,
			"medium":   true,
			"low":      true,
			"info":     true,
		}
		if !validSeverities[severity] {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf(": severity  critical、high、medium、low  info ，: %s", severity),
					},
				},
				IsError: true,
			}, nil
		}

		// English note.
		description := ""
		if d, ok := args["description"].(string); ok {
			description = d
		}

		vulnType := ""
		if t, ok := args["vulnerability_type"].(string); ok {
			vulnType = t
		}

		target := ""
		if t, ok := args["target"].(string); ok {
			target = t
		}

		proof := ""
		if p, ok := args["proof"].(string); ok {
			proof = p
		}

		impact := ""
		if i, ok := args["impact"].(string); ok {
			impact = i
		}

		recommendation := ""
		if r, ok := args["recommendation"].(string); ok {
			recommendation = r
		}

		// English note.
		vuln := &database.Vulnerability{
			ConversationID: conversationID,
			Title:          title,
			Description:    description,
			Severity:       severity,
			Status:         "open",
			Type:           vulnType,
			Target:         target,
			Proof:          proof,
			Impact:         impact,
			Recommendation: recommendation,
		}

		created, err := db.CreateVulnerability(vuln)
		if err != nil {
			logger.Error("", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf(": %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		logger.Info("",
			zap.String("id", created.ID),
			zap.String("title", created.Title),
			zap.String("severity", created.Severity),
			zap.String("conversation_id", conversationID),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("！\n\nID: %s\n: %s\n: %s\n: %s\n\n。", created.ID, created.Title, created.Severity, created.Status),
				},
			},
			IsError: false,
		}, nil
	}

	mcpServer.RegisterTool(tool, handler)
	logger.Info("")
}

// English note.
func registerWebshellTools(mcpServer *mcp.Server, db *database.DB, webshellHandler *handler.WebShellHandler, logger *zap.Logger) {
	if db == nil || webshellHandler == nil {
		logger.Warn(" WebShell ：db  webshellHandler ")
		return
	}

	// webshell_exec
	execTool := mcp.Tool{
		Name:             builtin.ToolWebshellExec,
		Description:      " WebShell ，。connection_id  AI 。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": "WebShell  ID（ ws_xxx）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
			},
			"required": []string{"connection_id", "command"},
		},
	}
	execHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		cmd, _ := args["command"].(string)
		if cid == "" || cmd == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id  command "}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: " WebShell "}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.ExecWithConnection(conn, cmd)
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		if !ok {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "HTTP  200，:\n" + output}}, IsError: false}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: false}, nil
	}
	mcpServer.RegisterTool(execTool, execHandler)

	// webshell_file_list
	listTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileList,
		Description:      " WebShell 。path （.）。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell  ID"},
				"path":          map[string]interface{}{"type": "string", "description": "， ."},
			},
			"required": []string{"connection_id"},
		},
	}
	listHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		if cid == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id "}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: " WebShell "}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "list", path, "", "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: !ok}, nil
	}
	mcpServer.RegisterTool(listTool, listHandler)

	// webshell_file_read
	readTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileRead,
		Description:      " WebShell 。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell  ID"},
				"path":          map[string]interface{}{"type": "string", "description": ""},
			},
			"required": []string{"connection_id", "path"},
		},
	}
	readHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		if cid == "" || path == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id  path "}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: " WebShell "}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "read", path, "", "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: !ok}, nil
	}
	mcpServer.RegisterTool(readTool, readHandler)

	// webshell_file_write
	writeTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileWrite,
		Description:      " WebShell （）。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell  ID"},
				"path":          map[string]interface{}{"type": "string", "description": ""},
				"content":       map[string]interface{}{"type": "string", "description": ""},
			},
			"required": []string{"connection_id", "path", "content"},
		},
	}
	writeHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		if cid == "" || path == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id  path "}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: " WebShell "}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "write", path, content, "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		if !ok {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "，:\n" + output}}, IsError: false}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "\n" + output}}, IsError: false}, nil
	}
	mcpServer.RegisterTool(writeTool, writeHandler)

	logger.Info("WebShell ")
}

// English note.
func registerWebshellManagementTools(mcpServer *mcp.Server, db *database.DB, webshellHandler *handler.WebShellHandler, logger *zap.Logger) {
	if db == nil {
		logger.Warn(" WebShell ：db ")
		return
	}

	// English note.
	listTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellList,
		Description:      " WebShell ，ID、URL、、。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
	listHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connections, err := db.ListWebshellConnections()
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: ": " + err.Error()}},
				IsError: true,
			}, nil
		}
		if len(connections) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell "}},
				IsError: false,
			}, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf(" %d  WebShell ：\n\n", len(connections)))
		for _, conn := range connections {
			sb.WriteString(fmt.Sprintf("ID: %s\n", conn.ID))
			sb.WriteString(fmt.Sprintf("  URL: %s\n", conn.URL))
			sb.WriteString(fmt.Sprintf("  : %s\n", conn.Type))
			sb.WriteString(fmt.Sprintf("  : %s\n", conn.Method))
			sb.WriteString(fmt.Sprintf("  : %s\n", conn.CmdParam))
			if conn.Remark != "" {
				sb.WriteString(fmt.Sprintf("  : %s\n", conn.Remark))
			}
			sb.WriteString(fmt.Sprintf("  : %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05")))
			sb.WriteString("\n")
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(listTool, listHandler)

	// English note.
	addTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellAdd,
		Description:      " WebShell 。 PHP、ASP、ASPX、JSP 。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Shell ， http://target.com/shell.php（）",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "/，/",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Shell ：php、asp、aspx、jsp， php",
					"enum":        []string{"php", "asp", "aspx", "jsp"},
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "：GET  POST， POST",
					"enum":        []string{"GET", "POST"},
				},
				"cmd_param": map[string]interface{}{
					"type":        "string",
					"description": "， cmd",
				},
				"remark": map[string]interface{}{
					"type":        "string",
					"description": "，",
				},
			},
			"required": []string{"url"},
		},
	}
	addHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		urlStr, _ := args["url"].(string)
		if urlStr == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: ": url "}},
				IsError: true,
			}, nil
		}

		password, _ := args["password"].(string)
		shellType, _ := args["type"].(string)
		if shellType == "" {
			shellType = "php"
		}
		method, _ := args["method"].(string)
		if method == "" {
			method = "post"
		}
		cmdParam, _ := args["cmd_param"].(string)
		if cmdParam == "" {
			cmdParam = "cmd"
		}
		remark, _ := args["remark"].(string)

		// English note.
		connID := "ws_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
		conn := &database.WebShellConnection{
			ID:        connID,
			URL:       urlStr,
			Password:  password,
			Type:      strings.ToLower(shellType),
			Method:    strings.ToLower(method),
			CmdParam:  cmdParam,
			Remark:    remark,
			CreatedAt: time.Now(),
		}

		if err := db.CreateWebshellConnection(conn); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell : " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell ！\n\nID: %s\nURL: %s\n: %s\n: %s\n: %s", conn.ID, conn.URL, conn.Type, conn.Method, conn.CmdParam),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(addTool, addHandler)

	// English note.
	updateTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellUpdate,
		Description:      " WebShell 。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": " WebShell  ID（）",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": " Shell ",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "/",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": " Shell ：php、asp、aspx、jsp",
					"enum":        []string{"php", "asp", "aspx", "jsp"},
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "：GET  POST",
					"enum":        []string{"GET", "POST"},
				},
				"cmd_param": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
				"remark": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	updateHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: ": connection_id "}},
				IsError: true,
			}, nil
		}

		// English note.
		existing, err := db.GetWebshellConnection(connID)
		if err != nil || existing == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell : " + connID}},
				IsError: true,
			}, nil
		}

		// English note.
		if urlStr, ok := args["url"].(string); ok && urlStr != "" {
			existing.URL = urlStr
		}
		if password, ok := args["password"].(string); ok {
			existing.Password = password
		}
		if shellType, ok := args["type"].(string); ok && shellType != "" {
			existing.Type = strings.ToLower(shellType)
		}
		if method, ok := args["method"].(string); ok && method != "" {
			existing.Method = strings.ToLower(method)
		}
		if cmdParam, ok := args["cmd_param"].(string); ok && cmdParam != "" {
			existing.CmdParam = cmdParam
		}
		if remark, ok := args["remark"].(string); ok {
			existing.Remark = remark
		}

		if err := db.UpdateWebshellConnection(existing); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell : " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell ！\n\nID: %s\nURL: %s\n: %s\n: %s\n: %s\n: %s", existing.ID, existing.URL, existing.Type, existing.Method, existing.CmdParam, existing.Remark),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(updateTool, updateHandler)

	// English note.
	deleteTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellDelete,
		Description:      " WebShell 。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": " WebShell  ID（）",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	deleteHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: ": connection_id "}},
				IsError: true,
			}, nil
		}

		if err := db.DeleteWebshellConnection(connID); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell : " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell  %s ", connID),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(deleteTool, deleteHandler)

	// English note.
	testTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellTest,
		Description:      " WebShell ，（ whoami  dir）。",
		ShortDescription: " WebShell ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": " WebShell  ID（）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "， whoami（Linux） dir（Windows）",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	testHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: ": connection_id "}},
				IsError: true,
			}, nil
		}

		// English note.
		conn, err := db.GetWebshellConnection(connID)
		if err != nil || conn == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: " WebShell : " + connID}},
				IsError: true,
			}, nil
		}

		// English note.
		testCmd, _ := args["command"].(string)
		if testCmd == "" {
			// English note.
			if conn.Type == "asp" || conn.Type == "aspx" {
				testCmd = "dir"
			} else {
				testCmd = "whoami"
			}
		}

		// English note.
		output, ok, errMsg := webshellHandler.ExecWithConnection(conn, testCmd)
		if errMsg != "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("！\n\nID: %s\nURL: %s\n: %s", connID, conn.URL, errMsg)}},
				IsError: true,
			}, nil
		}

		if !ok {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("！HTTP  200\n\nID: %s\nURL: %s\n: %s", connID, conn.URL, output)}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("！\n\nID: %s\nURL: %s\n: %s\n\n: %s\n:\n%s", connID, conn.URL, conn.Type, testCmd, output),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(testTool, testHandler)

	logger.Info("WebShell ")
}

// English note.
func initializeKnowledge(
	cfg *config.Config,
	db *database.DB,
	knowledgeDBConn *database.DB,
	mcpServer *mcp.Server,
	agentHandler *handler.AgentHandler,
	app *App, //  App 
	logger *zap.Logger,
) (*handler.KnowledgeHandler, error) {
	// English note.
	knowledgeDBPath := cfg.Database.KnowledgeDBPath
	var knowledgeDB *sql.DB

	if knowledgeDBPath != "" {
		// English note.
		// English note.
		if err := os.MkdirAll(filepath.Dir(knowledgeDBPath), 0755); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		var err error
		knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, logger)
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}
		knowledgeDB = knowledgeDBConn.DB
		logger.Info("", zap.String("path", knowledgeDBPath))
	} else {
		// English note.
		knowledgeDB = db.DB
		logger.Info("（knowledge_db_path）")
	}

	// English note.
	knowledgeManager := knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, logger)

	// English note.
	// English note.
	if cfg.Knowledge.Embedding.APIKey == "" {
		cfg.Knowledge.Embedding.APIKey = cfg.OpenAI.APIKey
	}
	if cfg.Knowledge.Embedding.BaseURL == "" {
		cfg.Knowledge.Embedding.BaseURL = cfg.OpenAI.BaseURL
	}

	embedder, err := knowledge.NewEmbedder(context.Background(), &cfg.Knowledge, &cfg.OpenAI, logger)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	retrievalConfig := &knowledge.RetrievalConfig{
		TopK:                cfg.Knowledge.Retrieval.TopK,
		SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
		SubIndexFilter:      cfg.Knowledge.Retrieval.SubIndexFilter,
		PostRetrieve:        cfg.Knowledge.Retrieval.PostRetrieve,
	}
	knowledgeRetriever := knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, logger)

	// English note.
	knowledgeIndexer, err := knowledge.NewIndexer(context.Background(), knowledgeDB, embedder, logger, &cfg.Knowledge)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, logger)

	// English note.
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, logger)
	logger.Info("", zap.Bool("handler_created", knowledgeHandler != nil))

	// English note.
	agentHandler.SetKnowledgeManager(knowledgeManager)

	// English note.
	if app != nil {
		app.knowledgeManager = knowledgeManager
		app.knowledgeRetriever = knowledgeRetriever
		app.knowledgeIndexer = knowledgeIndexer
		app.knowledgeHandler = knowledgeHandler
		// English note.
		if knowledgeDBPath != "" {
			app.knowledgeDB = knowledgeDBConn
		}
		logger.Info("App ")
	}

	// English note.
	go func() {
		itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
		if err != nil {
			logger.Warn("", zap.Error(err))
			return
		}

		// English note.
		hasIndex, err := knowledgeIndexer.HasIndex()
		if err != nil {
			logger.Warn("", zap.Error(err))
			return
		}

		if hasIndex {
			// English note.
			if len(itemsToIndex) > 0 {
				logger.Info("，", zap.Int("count", len(itemsToIndex)))
				ctx := context.Background()
				consecutiveFailures := 0
				var firstFailureItemID string
				var firstFailureError error
				failedCount := 0

				for _, itemID := range itemsToIndex {
					if err := knowledgeIndexer.IndexItem(ctx, itemID); err != nil {
						failedCount++
						consecutiveFailures++

						if consecutiveFailures == 1 {
							firstFailureItemID = itemID
							firstFailureError = err
							logger.Warn("", zap.String("itemId", itemID), zap.Error(err))
						}

						// English note.
						if consecutiveFailures >= 2 {
							logger.Error("，",
								zap.Int("consecutiveFailures", consecutiveFailures),
								zap.Int("totalItems", len(itemsToIndex)),
								zap.String("firstFailureItemId", firstFailureItemID),
								zap.Error(firstFailureError),
							)
							break
						}
						continue
					}

					// English note.
					if consecutiveFailures > 0 {
						consecutiveFailures = 0
						firstFailureItemID = ""
						firstFailureError = nil
					}
				}
				logger.Info("", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
			} else {
				logger.Info("，")
			}
			return
		}

		// English note.
		logger.Info("，")
		ctx := context.Background()
		if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
			logger.Warn("", zap.Error(err))
		}
	}()

	return knowledgeHandler, nil
}

// English note.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
