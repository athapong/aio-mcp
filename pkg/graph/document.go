package graph

// DocumentEntity represents a named entity extracted from text in a document
type DocumentEntity struct {
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Confidence float64                `json:"confidence"`
}

// DocumentRelationship represents a relationship between entities in a document
type DocumentRelationship struct {
	Type       string                 `json:"type"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Confidence float64                `json:"confidence"`
}
