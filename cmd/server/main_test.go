package main

import (
	"testing"

	"github.com/sasuke39/openWarpAdapter/internal/llm"

	"github.com/openai/openai-go"
)

func TestNormalizeConversationHistory_PrunesDanglingAssistantToolCallBeforeUserQuery(t *testing.T) {
	history := []openai.ChatCompletionMessageParamUnion{
		llm.MakeUserMessage("A"),
		llm.MakeAssistantToolCallMessage([]llm.ToolCall{
			{ID: "call-1", Name: "run_shell_command", Args: []byte(`{"command":"echo a"}`)},
		}, ""),
		llm.MakeUserMessage("B"),
	}

	normalized, changed := normalizeConversationHistory(history)
	if !changed {
		t.Fatalf("expected history normalization to report changes")
	}
	if got := len(normalized); got != 2 {
		t.Fatalf("expected 2 messages after pruning, got %d", got)
	}
	if normalized[0].OfUser == nil || normalized[1].OfUser == nil {
		t.Fatalf("expected remaining messages to be user messages")
	}
}

func TestNormalizeConversationHistory_PrunesIncompleteAssistantToolCallAndPartialToolResults(t *testing.T) {
	history := []openai.ChatCompletionMessageParamUnion{
		llm.MakeUserMessage("start"),
		llm.MakeAssistantToolCallMessage([]llm.ToolCall{
			{ID: "call-1", Name: "run_shell_command", Args: []byte(`{"command":"a"}`)},
			{ID: "call-2", Name: "run_shell_command", Args: []byte(`{"command":"b"}`)},
		}, ""),
		llm.MakeToolResultMessage("call-1", "ok"),
		llm.MakeUserMessage("next"),
	}

	normalized, changed := normalizeConversationHistory(history)
	if !changed {
		t.Fatalf("expected history normalization to report changes")
	}
	if got := len(normalized); got != 2 {
		t.Fatalf("expected 2 messages after pruning incomplete round, got %d", got)
	}
	if normalized[0].OfUser == nil || normalized[1].OfUser == nil {
		t.Fatalf("expected remaining messages to be user messages")
	}
}

func TestNormalizeConversationHistory_LeavesValidToolRoundIntact(t *testing.T) {
	history := []openai.ChatCompletionMessageParamUnion{
		llm.MakeUserMessage("start"),
		llm.MakeAssistantToolCallMessage([]llm.ToolCall{
			{ID: "call-1", Name: "run_shell_command", Args: []byte(`{"command":"a"}`)},
		}, ""),
		llm.MakeToolResultMessage("call-1", "ok"),
		llm.MakeUserMessage("next"),
	}

	normalized, changed := normalizeConversationHistory(history)
	if changed {
		t.Fatalf("expected valid history to remain unchanged")
	}
	if got := len(normalized); got != len(history) {
		t.Fatalf("expected %d messages, got %d", len(history), got)
	}
}

func TestNormalizeConversationHistory_LeavesValidToolRoundWithReasoningIntact(t *testing.T) {
	history := []openai.ChatCompletionMessageParamUnion{
		llm.MakeUserMessage("start"),
		llm.MakeAssistantToolCallMessage([]llm.ToolCall{
			{ID: "call-1", Name: "run_shell_command", Args: []byte(`{"command":"ls -la"}`)},
		}, "先看一下目录"),
		llm.MakeToolResultMessage("call-1", "ok"),
	}

	normalized, changed := normalizeConversationHistory(history)
	if changed {
		t.Fatalf("expected valid history with reasoning_content to remain unchanged")
	}
	if got := len(normalized); got != len(history) {
		t.Fatalf("expected %d messages, got %d", len(history), got)
	}
	if normalized[1].OfAssistant == nil {
		t.Fatalf("expected assistant tool call message to be preserved")
	}
	if normalized[2].OfTool == nil || normalized[2].OfTool.ToolCallID != "call-1" {
		t.Fatalf("expected tool result to remain paired with call-1")
	}
}

func TestNormalizeConversationHistory_PrunesStrayToolMessage(t *testing.T) {
	history := []openai.ChatCompletionMessageParamUnion{
		llm.MakeUserMessage("start"),
		llm.MakeToolResultMessage("call-1", "orphan"),
		llm.MakeUserMessage("next"),
	}

	normalized, changed := normalizeConversationHistory(history)
	if !changed {
		t.Fatalf("expected stray tool message to be pruned")
	}
	if got := len(normalized); got != 2 {
		t.Fatalf("expected 2 messages after pruning stray tool message, got %d", got)
	}
	if normalized[0].OfUser == nil || normalized[1].OfUser == nil {
		t.Fatalf("expected remaining messages to be user messages")
	}
}
