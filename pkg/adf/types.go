package adf

// Node represents an ADF node
type Node struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Marks   []*Mark                `json:"marks,omitempty"`
	Content []*Node                `json:"content,omitempty"`
}

// Mark represents formatting marks in ADF
type Mark struct {
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs,omitempty"`
}
