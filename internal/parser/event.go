package parser

import "time"

type ContentType int

const (
	ContentToolUse ContentType = iota
	ContentToolResult
	ContentText
	ContentThinking
	ContentUnknown
)

type ParsedEvent struct {
	SessionID    string
	RequestID    string
	UUID         string
	Timestamp    time.Time
	EntryType    string
	Model        string
	ContentType  ContentType
	ToolName     string
	ToolInput    string
	ToolResult   string
	IsError      bool
	ErrorMsg     string
	Tokens       TokenUsage
	FilesChanged []string
}

type TokenUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

func (t TokenUsage) Total() int {
	return t.InputTokens + t.OutputTokens + t.CacheReadTokens + t.CacheWriteTokens
}

type Parser interface {
	Parse(line []byte) ([]*ParsedEvent, error)
}
