package query

import (
	"encoding/json"
	"fmt"
)

type QueryType string

const (
	Match  QueryType = "MATCH"
	Create QueryType = "CREATE"
	Delete QueryType = "DELETE"
	Update QueryType = "UPDATE"
)

type Query struct {
	Type     QueryType `json:"type"`
	Patterns []Pattern `json:"patterns"`
	Filters  []Filter  `json:"filters"`
	Returns  []string  `json:"returns"`
	Limit    int       `json:"limit"`
	Skip     int       `json:"skip"`
}

type Pattern struct {
	NodeType     string                 `json:"node_type"`
	RelationType string                 `json:"relation_type,omitempty"`
	Direction    string                 `json:"direction,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

func NewQuery(queryType QueryType) *Query {
	return &Query{
		Type:     queryType,
		Patterns: make([]Pattern, 0),
		Filters:  make([]Filter, 0),
		Returns:  make([]string, 0),
	}
}

func (q *Query) AddPattern(pattern Pattern) *Query {
	q.Patterns = append(q.Patterns, pattern)
	return q
}

func (q *Query) AddFilter(filter Filter) *Query {
	q.Filters = append(q.Filters, filter)
	return q
}

func (q *Query) SetLimit(limit int) *Query {
	q.Limit = limit
	return q
}

func (q *Query) String() string {
	bytes, _ := json.MarshalIndent(q, "", "  ")
	return fmt.Sprintf("%s", bytes)
}
