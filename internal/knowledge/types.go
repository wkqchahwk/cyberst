package knowledge

import (
	"encoding/json"
	"time"
)

// English note.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// English note.
type KnowledgeItem struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // 风险类型（文件夹名）
	Title     string    `json:"title"`    // 标题（文件名）
	FilePath  string    `json:"filePath"` // 文件路径
	Content   string    `json:"content"`  // 文件内容
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// English note.
type KnowledgeItemSummary struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	FilePath  string    `json:"filePath"`
	Content   string    `json:"content,omitempty"` // 可选：内容预览（如果提供，通常只包含前 150 字符）
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// English note.
func (k *KnowledgeItemSummary) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeItemSummary
	aux := &struct {
		*Alias
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}{
		Alias: (*Alias)(k),
	}
	aux.CreatedAt = formatTime(k.CreatedAt)
	aux.UpdatedAt = formatTime(k.UpdatedAt)
	return json.Marshal(aux)
}

// English note.
func (k *KnowledgeItem) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeItem
	aux := &struct {
		*Alias
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}{
		Alias: (*Alias)(k),
	}
	aux.CreatedAt = formatTime(k.CreatedAt)
	aux.UpdatedAt = formatTime(k.UpdatedAt)
	return json.Marshal(aux)
}

// English note.
type KnowledgeChunk struct {
	ID         string    `json:"id"`
	ItemID     string    `json:"itemId"`
	ChunkIndex int       `json:"chunkIndex"`
	ChunkText  string    `json:"chunkText"`
	Embedding  []float32 `json:"-"` // 向量嵌入，不序列化到 JSON
	CreatedAt  time.Time `json:"createdAt"`
}

// English note.
type RetrievalResult struct {
	Chunk      *KnowledgeChunk `json:"chunk"`
	Item       *KnowledgeItem  `json:"item"`
	Similarity float64         `json:"similarity"` // 相似度分数
	Score      float64         `json:"score"`      // 与 Similarity 相同：余弦相似度
}

// English note.
type RetrievalLog struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	MessageID      string    `json:"messageId,omitempty"`
	Query          string    `json:"query"`
	RiskType       string    `json:"riskType,omitempty"`
	RetrievedItems []string  `json:"retrievedItems"` // 检索到的知识项 ID 列表
	CreatedAt      time.Time `json:"createdAt"`
}

// English note.
func (r *RetrievalLog) MarshalJSON() ([]byte, error) {
	type Alias RetrievalLog
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"createdAt"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: formatTime(r.CreatedAt),
	})
}

// English note.
type CategoryWithItems struct {
	Category  string                `json:"category"`           // 分类名称
	ItemCount int                   `json:"itemCount"`          // 该分类下的知识项总数
	Items     []*KnowledgeItemSummary `json:"items"`            // 该分类下的知识项列表
}

// English note.
type SearchRequest struct {
	Query          string  `json:"query"`
	RiskType       string  `json:"riskType,omitempty"`       // 可选：指定风险类型
	SubIndexFilter string  `json:"subIndexFilter,omitempty"` // 可选：仅保留 sub_indexes 含该标签的行（含未打标旧数据）
	TopK           int     `json:"topK,omitempty"`           // 返回 Top-K 结果，默认 5
	Threshold      float64 `json:"threshold,omitempty"`      // 相似度阈值，默认 0.7
}
