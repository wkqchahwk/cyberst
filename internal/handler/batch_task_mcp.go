package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

// English note.
func RegisterBatchTaskMCPTools(mcpServer *mcp.Server, h *AgentHandler, logger *zap.Logger) {
	if mcpServer == nil || h == nil || logger == nil {
		return
	}

	reg := func(tool mcp.Tool, fn func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error)) {
		mcpServer.RegisterTool(tool, fn)
	}

	// --- list ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskList,
		Description:      "（，）。、 id/status/ message、。（ result/error/conversationId/） batch_task_get(queue_id)。\n\n⚠️ ：「」，/、。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type":        "string",
					"description": "：all（）、pending、running、paused、completed、cancelled",
					"enum":        []string{"all", "pending", "running", "paused", "completed", "cancelled"},
				},
				"keyword": map[string]interface{}{
					"type":        "string",
					"description": " ID ",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "， 1 ， 1",
				},
				"page_size": map[string]interface{}{
					"type":        "integer",
					"description": "， 20， 100",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		status := mcpArgString(args, "status")
		if status == "" {
			status = "all"
		}
		keyword := mcpArgString(args, "keyword")
		page := int(mcpArgFloat(args, "page"))
		if page <= 0 {
			page = 1
		}
		pageSize := int(mcpArgFloat(args, "page_size"))
		if pageSize <= 0 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}
		offset := (page - 1) * pageSize
		if offset > 100000 {
			offset = 100000
		}
		queues, total, err := h.batchTaskManager.ListQueues(pageSize, offset, status, keyword)
		if err != nil {
			return batchMCPTextResult(fmt.Sprintf(": %v", err), true), nil
		}
		totalPages := (total + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		slim := make([]batchTaskQueueMCPListItem, 0, len(queues))
		for _, q := range queues {
			if q == nil {
				continue
			}
			slim = append(slim, toBatchTaskQueueMCPListItem(q))
		}
		payload := map[string]interface{}{
			"queues":      slim,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
		}
		logger.Info("MCP batch_task_list", zap.String("status", status), zap.Int("total", total))
		return batchMCPJSONResult(payload)
	})

	// --- get ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskGet,
		Description:      " queue_id （、Cron、）。\n\n⚠️ ：「」，/、。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		queue, ok := h.batchTaskManager.GetBatchQueue(qid)
		if !ok {
			return batchMCPTextResult(": "+qid, true), nil
		}
		return batchMCPJSONResult(queue)
	})

	// --- create ---
	reg(mcp.Tool{
		Name: builtin.ToolBatchTaskCreate,
		Description: `⚠️ ：「」，、。””””””。，，。

【】「 / 」：，、/、。，””。

【】、Cron 、。、/，，””。

【】tasks（） tasks_text（，）；。agent_mode：single（ ReAct，）、eino_single（Eino ADK ）、deep / plan_execute / supervisor（）； multi（ deep）。””。schedule_mode：manual（） cron；cron  cron_expr（5 ， “0 */6 * * *”）。

【】 pending，。execute_now=true ； batch_task_start。Cron  schedule_enabled  true（ batch_task_schedule_enabled）。`,
		ShortDescription: "：（， Cron）",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "，",
				},
				"role": map[string]interface{}{
					"type":        "string",
					"description": "，",
				},
				"tasks": map[string]interface{}{
					"type":        "array",
					"description": "，（ tasks_text ）",
					"items":       map[string]interface{}{"type": "string"},
				},
				"tasks_text": map[string]interface{}{
					"type":        "string",
					"description": "，（ tasks ）",
				},
				"agent_mode": map[string]interface{}{
					"type":        "string",
					"description": "：single（ ReAct）、eino_single（Eino ADK）、deep/plan_execute/supervisor（Eino ，）；multi  deep",
					"enum":        []string{"single", "eino_single", "deep", "plan_execute", "supervisor", "multi"},
				},
				"schedule_mode": map[string]interface{}{
					"type":        "string",
					"description": "manual（/） cron（）",
					"enum":        []string{"manual", "cron"},
				},
				"cron_expr": map[string]interface{}{
					"type":        "string",
					"description": "schedule_mode  cron 。 5 ：    ， \"0 */6 * * *\"、\"30 2 * * 1-5\"",
				},
				"execute_now": map[string]interface{}{
					"type":        "boolean",
					"description": "， false（pending， batch_task_start）",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		tasks, errMsg := batchMCPTasksFromArgs(args)
		if errMsg != "" {
			return batchMCPTextResult(errMsg, true), nil
		}
		title := mcpArgString(args, "title")
		role := mcpArgString(args, "role")
		agentMode := normalizeBatchQueueAgentMode(mcpArgString(args, "agent_mode"))
		scheduleMode := normalizeBatchQueueScheduleMode(mcpArgString(args, "schedule_mode"))
		cronExpr := strings.TrimSpace(mcpArgString(args, "cron_expr"))
		var nextRunAt *time.Time
		if scheduleMode == "cron" {
			if cronExpr == "" {
				return batchMCPTextResult("Cron  cron_expr ", true), nil
			}
			sch, err := h.batchCronParser.Parse(cronExpr)
			if err != nil {
				return batchMCPTextResult(" Cron : "+err.Error(), true), nil
			}
			n := sch.Next(time.Now())
			nextRunAt = &n
		}
		executeNow, ok := mcpArgBool(args, "execute_now")
		if !ok {
			executeNow = false
		}
		queue, createErr := h.batchTaskManager.CreateBatchQueue(title, role, agentMode, scheduleMode, cronExpr, nextRunAt, tasks)
		if createErr != nil {
			return batchMCPTextResult(": "+createErr.Error(), true), nil
		}
		started := false
		if executeNow {
			ok, err := h.startBatchQueueExecution(queue.ID, false)
			if !ok {
				return batchMCPTextResult(": "+queue.ID, true), nil
			}
			if err != nil {
				return batchMCPTextResult(": "+err.Error(), true), nil
			}
			started = true
			if refreshed, exists := h.batchTaskManager.GetBatchQueue(queue.ID); exists {
				queue = refreshed
			}
		}
		logger.Info("MCP batch_task_create", zap.String("queueId", queue.ID), zap.Int("taskCount", len(tasks)))
		return batchMCPJSONResult(map[string]interface{}{
			"queue_id":    queue.ID,
			"queue":       queue,
			"started":     started,
			"execute_now": executeNow,
			"reminder": func() string {
				if started {
					return "。"
				}
				return "， pending。 MCP  batch_task_start（queue_id ）。Cron  schedule_enabled  true， batch_task_schedule_enabled。"
			}(),
		})
	})

	// --- start ---
	reg(mcp.Tool{
		Name: builtin.ToolBatchTaskStart,
		Description: `（pending / paused）。
 batch_task_create ：，。

⚠️ ：「」，/。。`,
		ShortDescription: "/（）",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		ok, err := h.startBatchQueueExecution(qid, false)
		if !ok {
			return batchMCPTextResult(": "+qid, true), nil
		}
		if err != nil {
			return batchMCPTextResult(": "+err.Error(), true), nil
		}
		logger.Info("MCP batch_task_start", zap.String("queueId", qid))
		return batchMCPTextResult("，。", false), nil
	})

	// --- rerun (reset + start for completed/cancelled queues) ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskRerun,
		Description:      "。。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		queue, exists := h.batchTaskManager.GetBatchQueue(qid)
		if !exists {
			return batchMCPTextResult(": "+qid, true), nil
		}
		if queue.Status != "completed" && queue.Status != "cancelled" {
			return batchMCPTextResult("，: "+queue.Status, true), nil
		}
		if !h.batchTaskManager.ResetQueueForRerun(qid) {
			return batchMCPTextResult("", true), nil
		}
		ok, err := h.startBatchQueueExecution(qid, false)
		if !ok {
			return batchMCPTextResult("", true), nil
		}
		if err != nil {
			return batchMCPTextResult(": "+err.Error(), true), nil
		}
		logger.Info("MCP batch_task_rerun", zap.String("queueId", qid))
		return batchMCPTextResult("。", false), nil
	})

	// --- pause ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskPause,
		Description:      "（）。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		if !h.batchTaskManager.PauseQueue(qid) {
			return batchMCPTextResult("： running ", true), nil
		}
		logger.Info("MCP batch_task_pause", zap.String("queueId", qid))
		return batchMCPTextResult("。", false), nil
	})

	// --- delete queue ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskDelete,
		Description:      "。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		if !h.batchTaskManager.DeleteQueue(qid) {
			return batchMCPTextResult("：", true), nil
		}
		logger.Info("MCP batch_task_delete", zap.String("queueId", qid))
		return batchMCPTextResult("。", false), nil
	})

	// --- update metadata (title/role/agentMode) ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskUpdateMetadata,
		Description:      "、。 running 。\n\n⚠️ ：「」，。。",
		ShortDescription: "//",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "（）",
				},
				"role": map[string]interface{}{
					"type":        "string",
					"description": "（）",
				},
				"agent_mode": map[string]interface{}{
					"type":        "string",
					"description": "：single、eino_single、deep、plan_execute、supervisor；multi  deep",
					"enum":        []string{"single", "eino_single", "deep", "plan_execute", "supervisor", "multi"},
				},
			},
			"required": []string{"queue_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		title := mcpArgString(args, "title")
		role := mcpArgString(args, "role")
		agentMode := mcpArgString(args, "agent_mode")
		if err := h.batchTaskManager.UpdateQueueMetadata(qid, title, role, agentMode); err != nil {
			return batchMCPTextResult(err.Error(), true), nil
		}
		updated, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_update_metadata", zap.String("queueId", qid))
		return batchMCPJSONResult(updated)
	})

	// --- update schedule ---
	reg(mcp.Tool{
		Name: builtin.ToolBatchTaskUpdateSchedule,
		Description: ` Cron 。 running 。
schedule_mode  cron  cron_expr； manual  Cron 。

⚠️ ：「」，。。`,
		ShortDescription: "（Cron ）",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"schedule_mode": map[string]interface{}{
					"type":        "string",
					"description": "manual  cron",
					"enum":        []string{"manual", "cron"},
				},
				"cron_expr": map[string]interface{}{
					"type":        "string",
					"description": "Cron （schedule_mode  cron ）。 5 ：    ， \"0 */6 * * *\"（6）、\"30 2 * * 1-5\"（2:30）",
				},
			},
			"required": []string{"queue_id", "schedule_mode"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		queue, exists := h.batchTaskManager.GetBatchQueue(qid)
		if !exists {
			return batchMCPTextResult(": "+qid, true), nil
		}
		if queue.Status == "running" {
			return batchMCPTextResult("，", true), nil
		}
		scheduleMode := normalizeBatchQueueScheduleMode(mcpArgString(args, "schedule_mode"))
		cronExpr := strings.TrimSpace(mcpArgString(args, "cron_expr"))
		var nextRunAt *time.Time
		if scheduleMode == "cron" {
			if cronExpr == "" {
				return batchMCPTextResult("Cron  cron_expr ", true), nil
			}
			sch, err := h.batchCronParser.Parse(cronExpr)
			if err != nil {
				return batchMCPTextResult(" Cron : "+err.Error(), true), nil
			}
			n := sch.Next(time.Now())
			nextRunAt = &n
		}
		h.batchTaskManager.UpdateQueueSchedule(qid, scheduleMode, cronExpr, nextRunAt)
		updated, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_update_schedule", zap.String("queueId", qid), zap.String("scheduleMode", scheduleMode), zap.String("cronExpr", cronExpr))
		return batchMCPJSONResult(updated)
	})

	// --- schedule enabled ---
	reg(mcp.Tool{
		Name: builtin.ToolBatchTaskScheduleEnabled,
		Description: ` Cron 。 Cron ，；「」。
 schedule_mode  cron 。

⚠️ ：「」，。。`,
		ShortDescription: " Cron ",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"schedule_enabled": map[string]interface{}{
					"type":        "boolean",
					"description": "true ，false ",
				},
			},
			"required": []string{"queue_id", "schedule_enabled"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		if qid == "" {
			return batchMCPTextResult("queue_id ", true), nil
		}
		en, ok := mcpArgBool(args, "schedule_enabled")
		if !ok {
			return batchMCPTextResult("schedule_enabled ", true), nil
		}
		if _, exists := h.batchTaskManager.GetBatchQueue(qid); !exists {
			return batchMCPTextResult("", true), nil
		}
		if !h.batchTaskManager.SetScheduleEnabled(qid, en) {
			return batchMCPTextResult("", true), nil
		}
		queue, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_schedule_enabled", zap.String("queueId", qid), zap.Bool("enabled", en))
		return batchMCPJSONResult(queue)
	})

	// --- add task ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskAdd,
		Description:      " pending 。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
			},
			"required": []string{"queue_id", "message"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		msg := strings.TrimSpace(mcpArgString(args, "message"))
		if qid == "" || msg == "" {
			return batchMCPTextResult("queue_id  message ", true), nil
		}
		task, err := h.batchTaskManager.AddTaskToQueue(qid, msg)
		if err != nil {
			return batchMCPTextResult(err.Error(), true), nil
		}
		queue, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_add_task", zap.String("queueId", qid), zap.String("taskId", task.ID))
		return batchMCPJSONResult(map[string]interface{}{"task": task, "queue": queue})
	})

	// --- update task ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskUpdate,
		Description:      " pending  pending 。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "",
				},
			},
			"required": []string{"queue_id", "task_id", "message"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		tid := mcpArgString(args, "task_id")
		msg := strings.TrimSpace(mcpArgString(args, "message"))
		if qid == "" || tid == "" || msg == "" {
			return batchMCPTextResult("queue_id、task_id、message ", true), nil
		}
		if err := h.batchTaskManager.UpdateTaskMessage(qid, tid, msg); err != nil {
			return batchMCPTextResult(err.Error(), true), nil
		}
		queue, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_update_task", zap.String("queueId", qid), zap.String("taskId", tid))
		return batchMCPJSONResult(queue)
	})

	// --- remove task ---
	reg(mcp.Tool{
		Name:             builtin.ToolBatchTaskRemove,
		Description:      " pending  pending 。\n\n⚠️ ：「」，。。",
		ShortDescription: "",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"queue_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": " ID",
				},
			},
			"required": []string{"queue_id", "task_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		qid := mcpArgString(args, "queue_id")
		tid := mcpArgString(args, "task_id")
		if qid == "" || tid == "" {
			return batchMCPTextResult("queue_id  task_id ", true), nil
		}
		if err := h.batchTaskManager.DeleteTask(qid, tid); err != nil {
			return batchMCPTextResult(err.Error(), true), nil
		}
		queue, _ := h.batchTaskManager.GetBatchQueue(qid)
		logger.Info("MCP batch_task_remove_task", zap.String("queueId", qid), zap.String("taskId", tid))
		return batchMCPJSONResult(queue)
	})

	logger.Info(" MCP ", zap.Int("count", 12))
}

// English note.

const mcpBatchListTaskMessageMaxRunes = 160

// English note.
type batchTaskMCPListSummary struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// English note.
type batchTaskQueueMCPListItem struct {
	ID                    string                    `json:"id"`
	Title                 string                    `json:"title,omitempty"`
	Role                  string                    `json:"role,omitempty"`
	AgentMode             string                    `json:"agentMode"`
	ScheduleMode          string                    `json:"scheduleMode"`
	CronExpr              string                    `json:"cronExpr,omitempty"`
	NextRunAt             *time.Time                `json:"nextRunAt,omitempty"`
	ScheduleEnabled       bool                      `json:"scheduleEnabled"`
	LastScheduleTriggerAt *time.Time                `json:"lastScheduleTriggerAt,omitempty"`
	Status                string                    `json:"status"`
	CreatedAt             time.Time                 `json:"createdAt"`
	StartedAt             *time.Time                `json:"startedAt,omitempty"`
	CompletedAt           *time.Time                `json:"completedAt,omitempty"`
	CurrentIndex          int                       `json:"currentIndex"`
	TaskTotal             int                       `json:"task_total"`
	TaskCounts            map[string]int            `json:"task_counts"`
	Tasks                 []batchTaskMCPListSummary `json:"tasks"`
}

func truncateStringRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			out := strings.TrimSpace(s[:i])
			if out == "" {
				return "…"
			}
			return out + "…"
		}
		n++
	}
	return s
}

const mcpBatchListMaxTasksPerQueue = 200 // 

func toBatchTaskQueueMCPListItem(q *BatchTaskQueue) batchTaskQueueMCPListItem {
	counts := map[string]int{
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
	}
	tasks := make([]batchTaskMCPListSummary, 0, len(q.Tasks))
	for _, t := range q.Tasks {
		if t == nil {
			continue
		}
		counts[t.Status]++
		// English note.
		if len(tasks) < mcpBatchListMaxTasksPerQueue {
			tasks = append(tasks, batchTaskMCPListSummary{
				ID:      t.ID,
				Status:  t.Status,
				Message: truncateStringRunes(t.Message, mcpBatchListTaskMessageMaxRunes),
			})
		}
	}
	return batchTaskQueueMCPListItem{
		ID:                    q.ID,
		Title:                 q.Title,
		Role:                  q.Role,
		AgentMode:             q.AgentMode,
		ScheduleMode:          q.ScheduleMode,
		CronExpr:              q.CronExpr,
		NextRunAt:             q.NextRunAt,
		ScheduleEnabled:       q.ScheduleEnabled,
		LastScheduleTriggerAt: q.LastScheduleTriggerAt,
		Status:                q.Status,
		CreatedAt:             q.CreatedAt,
		StartedAt:             q.StartedAt,
		CompletedAt:           q.CompletedAt,
		CurrentIndex:          q.CurrentIndex,
		TaskTotal:             len(tasks),
		TaskCounts:            counts,
		Tasks:                 tasks,
	}
}

func batchMCPTextResult(text string, isErr bool) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.Content{{Type: "text", Text: text}},
		IsError: isErr,
	}
}

func batchMCPJSONResult(v interface{}) (*mcp.ToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return batchMCPTextResult(fmt.Sprintf("JSON : %v", err), true), nil
	}
	return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: string(b)}}}, nil
}

func batchMCPTasksFromArgs(args map[string]interface{}) ([]string, string) {
	if raw, ok := args["tasks"]; ok && raw != nil {
		switch t := raw.(type) {
		case []interface{}:
			out := make([]string, 0, len(t))
			for _, x := range t {
				if s, ok := x.(string); ok {
					if tr := strings.TrimSpace(s); tr != "" {
						out = append(out, tr)
					}
				}
			}
			if len(out) > 0 {
				return out, ""
			}
		}
	}
	if txt := mcpArgString(args, "tasks_text"); txt != "" {
		lines := strings.Split(txt, "\n")
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			if tr := strings.TrimSpace(line); tr != "" {
				out = append(out, tr)
			}
		}
		if len(out) > 0 {
			return out, ""
		}
	}
	return nil, " tasks（） tasks_text（，）"
}

func mcpArgString(args map[string]interface{}, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(t, 'f', -1, 64))
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func mcpArgFloat(args map[string]interface{}, key string) float64 {
	v, ok := args[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

func mcpArgBool(args map[string]interface{}, key string) (val bool, ok bool) {
	v, exists := args[key]
	if !exists {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		if s == "true" || s == "1" || s == "yes" {
			return true, true
		}
		if s == "false" || s == "0" || s == "no" {
			return false, true
		}
	case float64:
		return t != 0, true
	}
	return false, false
}
