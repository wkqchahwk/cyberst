package handler

// English note.
// English note.

var apiDocI18nTagToKey = map[string]string{
	"Auth": "auth", "Conversation Management": "conversationManagement", "Conversation Interaction": "conversationInteraction",
	"Batch Tasks": "batchTasks", "Conversation Groups": "conversationGroups", "Vulnerability Management": "vulnerabilityManagement",
	"Role Management": "roleManagement", "Skills Management": "skillsManagement", "Monitoring": "monitoring",
	"Config Management": "configManagement", "External MCP Management": "externalMCPManagement", "Attack Chain": "attackChain",
	"Knowledge Base": "knowledgeBase", "MCP": "mcp",
	"Fofa Recon": "fofaRecon", "Terminal": "terminal", "Webshell Management": "webshellManagement",
	"Chat Uploads": "chatUploads", "Robot Integration": "robotIntegration", "Markdown Agents": "markdownAgents",
}

var apiDocI18nSummaryToKey = map[string]string{
	"Login": "login", "Logout": "logout", "Change Password": "changePassword", "Validate Token": "validateToken",
	"Create Conversation": "createConversation", "List Conversations": "listConversations", "Get Conversation Detail": "getConversationDetail",
	"Update Conversation": "updateConversation", "Delete Conversation": "deleteConversation", "Get Conversation Result": "getConversationResult",
	"Send Message Non Stream": "sendMessageNonStream", "Send Message Stream": "sendMessageStream",
	"Cancel Task": "cancelTask", "List Running Tasks": "listRunningTasks", "List Completed Tasks": "listCompletedTasks",
	"Create Batch Queue": "createBatchQueue", "List Batch Queues": "listBatchQueues", "Get Batch Queue": "getBatchQueue",
	"Delete Batch Queue": "deleteBatchQueue", "Start Batch Queue": "startBatchQueue", "Pause Batch Queue": "pauseBatchQueue",
	"Add Task To Queue": "addTaskToQueue", "Sql Injection Scan": "sqlInjectionScan", "Port Scan": "portScan",
	"Update Batch Task": "updateBatchTask", "Delete Batch Task": "deleteBatchTask",
	"Create Group": "createGroup", "List Groups": "listGroups", "Get Group": "getGroup", "Update Group": "updateGroup",
	"Delete Group": "deleteGroup", "Get Group Conversations": "getGroupConversations", "Add Conversation To Group": "addConversationToGroup",
	"Remove Conversation From Group": "removeConversationFromGroup",
	"List Vulnerabilities": "listVulnerabilities", "Create Vulnerability": "createVulnerability", "Get Vulnerability Stats": "getVulnerabilityStats",
	"Get Vulnerability": "getVulnerability", "Update Vulnerability": "updateVulnerability", "Delete Vulnerability": "deleteVulnerability",
	"List Roles": "listRoles", "Create Role": "createRole", "Get Role": "getRole", "Update Role": "updateRole", "Delete Role": "deleteRole",
	"Get Available Skills": "getAvailableSkills", "List Skills": "listSkills", "Create Skill": "createSkill",
	"Get Skill Stats": "getSkillStats", "Clear Skill Stats": "clearSkillStats", "Get Skill": "getSkill",
	"Update Skill": "updateSkill", "Delete Skill": "deleteSkill", "Get Bound Roles": "getBoundRoles",
	"Get Monitor Info": "getMonitorInfo", "Get Execution Records": "getExecutionRecords", "Delete Execution Record": "deleteExecutionRecord",
	"Batch Delete Execution Records": "batchDeleteExecutionRecords", "Get Stats": "getStats",
	"Get Config": "getConfig", "Update Config": "updateConfig", "Get Tool Config": "getToolConfig", "Apply Config": "applyConfig",
	"List External MCP": "listExternalMCP", "Get External MCP Stats": "getExternalMCPStats", "Get External MCP": "getExternalMCP",
	"Add Or Update External MCP": "addOrUpdateExternalMCP", "Stdio Mode Config": "stdioModeConfig", "Sse Mode Config": "sseModeConfig",
	"Delete External MCP": "deleteExternalMCP", "Start External MCP": "startExternalMCP", "Stop External MCP": "stopExternalMCP",
	"Get Attack Chain": "getAttackChain", "Regenerate Attack Chain": "regenerateAttackChain",
	"Pin Conversation": "pinConversation", "Pin Group": "pinGroup", "Pin Group Conversation": "pinGroupConversation",
	"Get Categories": "getCategories", "List Knowledge Items": "listKnowledgeItems", "Create Knowledge Item": "createKnowledgeItem",
	"Get Knowledge Item": "getKnowledgeItem", "Update Knowledge Item": "updateKnowledgeItem", "Delete Knowledge Item": "deleteKnowledgeItem",
	"Get Index Status": "getIndexStatus", "Rebuild Index": "rebuildIndex", "Scan Knowledge Base": "scanKnowledgeBase",
	"Search Knowledge Base": "searchKnowledgeBase", "Basic Search": "basicSearch", "Search By Risk Type": "searchByRiskType",
	"Get Retrieval Logs": "getRetrievalLogs", "Delete Retrieval Log": "deleteRetrievalLog",
	"Mcp Endpoint": "mcpEndpoint", "List All Tools": "listAllTools", "Invoke Tool": "invokeTool", "Init Connection": "initConnection",
	"Success Response": "successResponse", "Error Response": "errorResponse",
	// English note.
	"Delete Conversation Turn": "deleteConversationTurn", "Get Message Process Details": "getMessageProcessDetails",
	"Rerun Batch Queue": "rerunBatchQueue", "Update Batch Queue Metadata": "updateBatchQueueMetadata",
	"Update Batch Queue Schedule": "updateBatchQueueSchedule", "Set Batch Queue Schedule Enabled": "setBatchQueueScheduleEnabled",
	"Get All Group Mappings": "getAllGroupMappings",
	"Fofa Search": "fofaSearch", "Fofa Parse": "fofaParse",
	"Test Open AI": "testOpenAI",
	"Terminal Run": "terminalRun", "Terminal Run Stream": "terminalRunStream", "Terminal WS": "terminalWS",
	"List Webshell Connections": "listWebshellConnections", "Create Webshell Connection": "createWebshellConnection",
	"Update Webshell Connection": "updateWebshellConnection", "Delete Webshell Connection": "deleteWebshellConnection",
	"Get Webshell Connection State": "getWebshellConnectionState", "Save Webshell Connection State": "saveWebshellConnectionState",
	"Get Webshell AIHistory": "getWebshellAIHistory", "List Webshell AIConversations": "listWebshellAIConversations",
	"Webshell Exec": "webshellExec", "Webshell File Op": "webshellFileOp",
	"List Chat Uploads": "listChatUploads", "Upload Chat File": "uploadChatFile", "Delete Chat Upload": "deleteChatUpload",
	"Download Chat Upload": "downloadChatUpload", "Get Chat Upload Content": "getChatUploadContent",
	"Put Chat Upload Content": "putChatUploadContent", "Mkdir Chat Upload": "mkdirChatUpload", "Rename Chat Upload": "renameChatUpload",
	"Wecom Callback Verify": "wecomCallbackVerify", "Wecom Callback Message": "wecomCallbackMessage",
	"Dingtalk Callback": "dingtalkCallback", "Lark Callback": "larkCallback", "Test Robot": "testRobot",
	"List Markdown Agents": "listMarkdownAgents", "Create Markdown Agent": "createMarkdownAgent",
	"Get Markdown Agent": "getMarkdownAgent", "Update Markdown Agent": "updateMarkdownAgent", "Delete Markdown Agent": "deleteMarkdownAgent",
	"List Skill Package Files": "listSkillPackageFiles", "Get Skill Package File": "getSkillPackageFile", "Put Skill Package File": "putSkillPackageFile",
	"Batch Get Tool Names": "batchGetToolNames",
	"Get Knowledge Stats": "getKnowledgeStats",
}

var apiDocI18nResponseDescToKey = map[string]string{
	"Get Success": "getSuccess", "Unauthorized": "unauthorized", "Unauthorized Token": "unauthorizedToken",
	"Create Success": "createSuccess", "Bad Request": "badRequest", "Conversation Not Found": "conversationNotFound",
	"Conversation Or Result Not Found": "conversationOrResultNotFound", "Bad Request Task Empty": "badRequestTaskEmpty",
	"Bad Request Group Name Exists": "badRequestGroupNameExists", "Group Not Found": "groupNotFound",
	"Bad Request Config": "badRequestConfig",
	"Bad Request Query Empty": "badRequestQueryEmpty", "Method Not Allowed": "methodNotAllowed",
	"Login Success": "loginSuccess", "Invalid Password": "invalidPassword", "Logout Success": "logoutSuccess",
	"Password Changed": "passwordChanged", "Token Valid": "tokenValid", "Token Invalid": "tokenInvalid",
	"Conversation Created": "conversationCreated", "Internal Error": "internalError", "Update Success": "updateSuccess",
	"Delete Success": "deleteSuccess", "Queue Not Found": "queueNotFound", "Start Success": "startSuccess",
	"Pause Success": "pauseSuccess", "Add Success": "addSuccess",
	"Task Not Found": "taskNotFound", "Conversation Or Group Not Found": "conversationOrGroupNotFound",
	"Cancel Submitted": "cancelSubmitted", "No Running Task": "noRunningTask",
	"Message Sent": "messageSent", "Stream Response": "streamResponse",
	// English note.
	"Bad Request Or Delete Failed": "badRequestOrDeleteFailed",
	"Param Error": "paramError", "Only Completed Or Cancelled Can Rerun": "onlyCompletedOrCancelledCanRerun",
	"Bad Request Or Queue Running": "badRequestOrQueueRunning", "Set Success": "setSuccess",
	"Search Success": "searchSuccess", "Parse Success": "parseSuccess", "Test Result": "testResult",
	"Execution Done": "executionDone", "Sse Event Stream": "sseEventStream", "Ws Established": "wsEstablished",
	"File Download": "fileDownload", "File Not Found": "fileNotFound", "Write Success": "writeSuccess",
	"Rename Success": "renameSuccess", "Wecom Verify Success": "wecomVerifySuccess",
	"Process Success": "processSuccess", "Agent Not Found": "agentNotFound", "Save Success": "saveSuccess",
	"Operation Result": "operationResult", "Execution Result": "executionResult", "Connection Not Found": "connectionNotFound",
}

// English note.
// English note.
func enrichSpecWithI18nKeys(spec map[string]interface{}) {
	paths, _ := spec["paths"].(map[string]interface{})
	if paths == nil {
		return
	}
	for _, pathItem := range paths {
		pm, _ := pathItem.(map[string]interface{})
		if pm == nil {
			continue
		}
		for _, method := range []string{"get", "post", "put", "delete", "patch"} {
			opVal, ok := pm[method]
			if !ok {
				continue
			}
			op, _ := opVal.(map[string]interface{})
			if op == nil {
				continue
			}
			// English note.
			switch tags := op["tags"].(type) {
			case []string:
				if len(tags) > 0 {
					keys := make([]string, 0, len(tags))
					for _, s := range tags {
						if k := apiDocI18nTagToKey[s]; k != "" {
							keys = append(keys, k)
						} else {
							keys = append(keys, s)
						}
					}
					op["x-i18n-tags"] = keys
				}
			case []interface{}:
				if len(tags) > 0 {
					keys := make([]interface{}, 0, len(tags))
					for _, t := range tags {
						if s, ok := t.(string); ok {
							if k := apiDocI18nTagToKey[s]; k != "" {
								keys = append(keys, k)
							} else {
								keys = append(keys, s)
							}
						}
					}
					if len(keys) > 0 {
						op["x-i18n-tags"] = keys
					}
				}
			}
			// x-i18n-summary
			if summary, _ := op["summary"].(string); summary != "" {
				if k := apiDocI18nSummaryToKey[summary]; k != "" {
					op["x-i18n-summary"] = k
				}
			}
			// English note.
			if respMap, _ := op["responses"].(map[string]interface{}); respMap != nil {
				for _, rv := range respMap {
					if r, _ := rv.(map[string]interface{}); r != nil {
						if desc, _ := r["description"].(string); desc != "" {
							if k := apiDocI18nResponseDescToKey[desc]; k != "" {
								r["x-i18n-description"] = k
							}
						}
					}
				}
			}
		}
	}
}
