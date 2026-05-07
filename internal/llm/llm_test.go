package llm

import (
	"testing"

	"github.com/openai/openai-go"
)

func TestExtractToolCalls_UsesStableProviderIndex(t *testing.T) {
	chunks := []openai.ChatCompletionChunk{
		{
			Choices: []openai.ChatCompletionChunkChoice{
				{
					Delta: openai.ChatCompletionChunkChoiceDelta{
						ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{
							{
								Index: 1,
								ID:    "call-b",
								Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
									Name:      "run_shell_command",
									Arguments: `{"command":"b`,
								},
							},
						},
					},
				},
			},
		},
		{
			Choices: []openai.ChatCompletionChunkChoice{
				{
					Delta: openai.ChatCompletionChunkChoiceDelta{
						ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{
							{
								Index: 0,
								ID:    "call-a",
								Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
									Name:      "read_files",
									Arguments: `{"files":["a"]}`,
								},
							},
							{
								Index: 1,
								Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
									Arguments: `"}`,
								},
							},
						},
					},
				},
			},
		},
	}

	toolCalls := ExtractToolCalls(chunks)
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call-b" || toolCalls[0].Name != "run_shell_command" || string(toolCalls[0].Args) != `{"command":"b"}` {
		t.Fatalf("unexpected first tool call: %+v", toolCalls[0])
	}
	if toolCalls[1].ID != "call-a" || toolCalls[1].Name != "read_files" || string(toolCalls[1].Args) != `{"files":["a"]}` {
		t.Fatalf("unexpected second tool call: %+v", toolCalls[1])
	}
}
