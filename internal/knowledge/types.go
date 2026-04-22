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
	Category  string    `json:"category"` // （）
	Title     string    `json:"title"`    // （）
	FilePath  string    `json:"filePath"` // 
	Content   string    `json:"content"`  // 
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// English note.
type KnowledgeItemSummary struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	FilePath  string    `json:"filePath"`
	Content   string    `json:"content,omitempty"` // ：（， 150 ）
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
	Embedding  []float32 `json:"-"` // ， JSON
	CreatedAt  time.Time `json:"createdAt"`
}

// English note.
type RetrievalResult struct {
	Chunk      *KnowledgeChunk `json:"chunk"`
	Item       *KnowledgeItem  `json:"item"`
	Similarity float64         `json:"similarity"` // 
	Score      float64         `json:"score"`      //  Similarity ：
}

// English note.
type RetrievalLog struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	MessageID      string    `json:"messageId,omitempty"`
	Query          string    `json:"query"`
	RiskType       string    `json:"riskType,omitempty"`
	RetrievedItems []string  `json:"retrievedItems"` //  ID 
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
	Category  string                `json:"category"`           // 
	ItemCount int                   `json:"itemCount"`          // 
	Items     []*KnowledgeItemSummary `json:"items"`            // 
}

// English note.
type SearchRequest struct {
	Query          string  `json:"query"`
	RiskType       string  `json:"riskType,omitempty"`       // ：
	SubIndexFilter string  `json:"subIndexFilter,omitempty"` // ： sub_indexes （）
	TopK           int     `json:"topK,omitempty"`           //  Top-K ， 5
	Threshold      float64 `json:"threshold,omitempty"`      // ， 0.7
}
