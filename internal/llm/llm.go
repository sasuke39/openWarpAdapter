package llm

import (
	"context"
	"encoding/json"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/sasuke39/openWarpAdapter/internal/config"
)

type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// StreamResult holds the collected output from a streaming LLM response.
type StreamResult struct {
	Text             string
	ReasoningContent string
	ToolCalls        []ToolCall
	IsToolCall       bool
}

type Client struct {
	client *openai.Client
	model  string
	tools  []openai.ChatCompletionToolParam
}

func NewClient(cfg *config.Config) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	opts = append(opts, option.WithBaseURL(baseURL))
	log.Printf("[LLM] NewClient: base_url=%s model=%s key_len=%d", baseURL, cfg.Model, len(cfg.APIKey))

	client := openai.NewClient(opts...)

	tools := []openai.ChatCompletionToolParam{
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "read_files",
				Description: param.NewOpt("Read the contents of files. Provide file paths and optional line ranges."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{"type": "string", "description": "File path"},
									"line_ranges": map[string]any{
										"type": "array",
										"items": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"start": map[string]any{"type": "integer"},
												"end":   map[string]any{"type": "integer"},
											},
										},
									},
								},
								"required": []string{"name"},
							},
						},
					},
					"required": []string{"files"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "grep",
				Description: param.NewOpt("Search for patterns in files using regular expressions."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"queries": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Search patterns (regex supported)",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Directory or file to search in",
						},
					},
					"required": []string{"queries"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "file_glob",
				Description: param.NewOpt("Find files matching glob patterns."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"patterns": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": `Glob patterns to match (e.g. ["*.go", "src/**/*.rs"])`,
						},
						"search_dir": map[string]any{
							"type":        "string",
							"description": "Directory to search in",
						},
					},
					"required": []string{"patterns"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "run_shell_command",
				Description: param.NewOpt("Execute a shell command in the terminal. Use this to run build commands, tests, git operations, or any CLI tool."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute",
						},
						"is_read_only": map[string]any{
							"type":        "boolean",
							"description": "Whether this command is read-only (no side effects)",
						},
						"is_risky": map[string]any{
							"type":        "boolean",
							"description": "Whether this command is potentially risky/destructive",
						},
						"risk_category": map[string]any{
							"type":        "string",
							"enum":        []string{"RISK_CATEGORY_READ_ONLY", "RISK_CATEGORY_TRIVIAL_LOCAL_CHANGE", "RISK_CATEGORY_NONTRIVIAL_LOCAL_CHANGE", "RISK_CATEGORY_EXTERNAL_CHANGE", "RISK_CATEGORY_RISKY"},
							"description": "Risk classification: READ_ONLY (no changes), TRIVIAL_LOCAL_CHANGE (minor file edit), NONTRIVIAL_LOCAL_CHANGE (significant file edit), EXTERNAL_CHANGE (network/external effects), RISKY (destructive)",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "read_shell_command_output",
				Description: param.NewOpt("Continue waiting for a previously started long-running shell command, or fetch more output from it. Use the command_id from a prior 'Command still running' result."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"command_id": map[string]any{
							"type":        "string",
							"description": "The command ID from a previous long-running shell command result",
						},
						"wait_for_completion": map[string]any{
							"type":        "boolean",
							"description": "Whether to wait until the command completes before returning",
						},
						"duration_seconds": map[string]any{
							"type":        "integer",
							"description": "If not waiting for completion, poll again after this many seconds",
						},
					},
					"required": []string{"command_id"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "transfer_shell_command_control_to_user",
				Description: param.NewOpt("Hand control of a still-running shell command back to the user when the command is interactive or needs manual intervention."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"reason": map[string]any{
							"type":        "string",
							"description": "Why the user should take over this running command",
						},
					},
					"required": []string{"reason"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "apply_file_diffs",
				Description: param.NewOpt("Apply changes to files: create new files, edit existing ones, or delete files. Use this to implement code changes."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"summary": map[string]any{
							"type":        "string",
							"description": "A short summary of what these changes accomplish",
						},
						"diffs": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"file_path": map[string]any{"type": "string", "description": "Path to the file to edit"},
									"search":    map[string]any{"type": "string", "description": "Content to search for and replace"},
									"replace":   map[string]any{"type": "string", "description": "Replacement content"},
								},
								"required": []string{"file_path", "search", "replace"},
							},
							"description": "List of file edits (search/replace pairs)",
						},
						"new_files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"file_path": map[string]any{"type": "string", "description": "Path for the new file"},
									"content":   map[string]any{"type": "string", "description": "Full contents of the new file"},
								},
								"required": []string{"file_path", "content"},
							},
							"description": "List of new files to create",
						},
						"deleted_files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"file_path": map[string]any{"type": "string", "description": "Path to the file to delete"},
								},
								"required": []string{"file_path"},
							},
							"description": "List of files to delete",
						},
					},
					"required": []string{"summary"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "file_glob_v2",
				Description: param.NewOpt("Find files matching glob patterns with advanced options like max depth and max matches."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"patterns": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Glob patterns to match (e.g. [\"*.go\", \"src/**/*.rs\"])",
						},
						"search_dir": map[string]any{
							"type":        "string",
							"description": "Directory to search in",
						},
						"max_matches": map[string]any{
							"type":        "integer",
							"description": "Maximum number of matches to return (0 = unlimited)",
						},
						"max_depth": map[string]any{
							"type":        "integer",
							"description": "Maximum directory depth to search (0 = unlimited)",
						},
						"min_depth": map[string]any{
							"type":        "integer",
							"description": "Minimum directory depth to search (0 = unlimited)",
						},
					},
					"required": []string{"patterns"},
				},
			},
		},
		{
			Function: shared.FunctionDefinitionParam{
				Name:        "search_codebase",
				Description: param.NewOpt("Semantically search the codebase for relevant code. Use this for high-level questions about how the code works."),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Natural language query describing what you're looking for",
						},
						"path_filters": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Optional file path patterns to filter by",
						},
						"codebase_path": map[string]any{
							"type":        "string",
							"description": "Optional absolute path to the codebase root",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	return &Client{
		client: &client,
		model:  cfg.Model,
		tools:  tools,
	}
}

func (c *Client) StreamChat(ctx context.Context, systemPrompt string, history []openai.ChatCompletionMessageParamUnion) *ssestream.Stream[openai.ChatCompletionChunk] {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+1)
	msgs = append(msgs, openai.SystemMessage(systemPrompt))
	msgs = append(msgs, history...)

	log.Printf("[LLM] StreamChat: model=%s msg_count=%d tools=%d", c.model, len(msgs), len(c.tools))
	return c.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.model),
		Messages: msgs,
		Tools:    c.tools,
	})
}

func MakeUserMessage(content string) openai.ChatCompletionMessageParamUnion {
	return openai.UserMessage(content)
}

func MakeToolResultMessage(toolCallID, content string) openai.ChatCompletionMessageParamUnion {
	return openai.ToolMessage(content, toolCallID)
}

func MakeAssistantToolCallMessage(toolCalls []ToolCall, reasoningContent string) openai.ChatCompletionMessageParamUnion {
	tcs := make([]openai.ChatCompletionMessageToolCallParam, 0, len(toolCalls))
	for _, tc := range toolCalls {
		tcs = append(tcs, openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Name,
				Arguments: string(tc.Args),
			},
		})
	}

	if reasoningContent != "" {
		msg := map[string]any{
			"role":              "assistant",
			"tool_calls":        tcs,
			"reasoning_content": reasoningContent,
		}
		raw, _ := json.Marshal(msg)
		overridden := param.Override[openai.ChatCompletionAssistantMessageParam](json.RawMessage(raw))
		return openai.ChatCompletionMessageParamUnion{
			OfAssistant: &overridden,
		}
	}

	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			ToolCalls: tcs,
		},
	}
}

// MakeAssistantMessageWithReasoning builds an assistant message that includes
// reasoning_content for DeepSeek thinking models. It uses raw JSON injection
// because the openai-go SDK doesn't support this field.
func MakeAssistantMessageWithReasoning(text, reasoningContent string) openai.ChatCompletionMessageParamUnion {
	if reasoningContent == "" {
		return openai.AssistantMessage(text)
	}
	// Build raw JSON with reasoning_content field, then use Override to inject it
	msg := map[string]any{
		"role":              "assistant",
		"content":           text,
		"reasoning_content": reasoningContent,
	}
	raw, _ := json.Marshal(msg)
	overridden := param.Override[openai.ChatCompletionAssistantMessageParam](json.RawMessage(raw))
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &overridden,
	}
}

func IsToolCallFinish(chunks []openai.ChatCompletionChunk) bool {
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.FinishReason == "tool_calls" {
				return true
			}
		}
	}
	return false
}

func ExtractToolCalls(chunks []openai.ChatCompletionChunk) []ToolCall {
	calls := map[int64]*ToolCall{}
	order := []int64{}

	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index
				if _, ok := calls[idx]; !ok {
					calls[idx] = &ToolCall{}
					order = append(order, idx)
				}
				if tc.ID != "" {
					calls[idx].ID = tc.ID
				}
				if tc.Function.Name != "" {
					calls[idx].Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					calls[idx].Args = append(calls[idx].Args, tc.Function.Arguments...)
				}
			}
		}
	}

	result := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		result = append(result, *calls[idx])
	}
	return result
}

func CollectTextDeltas(chunks []openai.ChatCompletionChunk) string {
	var text string
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			text += choice.Delta.Content
		}
	}
	return text
}

func CollectStreamResult(chunks []openai.ChatCompletionChunk) StreamResult {
	var result StreamResult
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			result.Text += choice.Delta.Content
			// DeepSeek reasoning models return reasoning_content in delta.
			// Try ExtraFields first, then fall back to RawJSON parsing.
			if f, ok := choice.Delta.JSON.ExtraFields["reasoning_content"]; ok && f.Valid() {
				var s string
				if err := json.Unmarshal([]byte(f.Raw()), &s); err == nil {
					result.ReasoningContent += s
				}
			} else if raw := choice.Delta.RawJSON(); raw != "" {
				var delta map[string]json.RawMessage
				if err := json.Unmarshal([]byte(raw), &delta); err == nil {
					if rc, ok := delta["reasoning_content"]; ok {
						var s string
						if json.Unmarshal(rc, &s) == nil {
							result.ReasoningContent += s
						}
					}
				}
			}
		}
	}
	if IsToolCallFinish(chunks) {
		result.IsToolCall = true
		result.ToolCalls = ExtractToolCalls(chunks)
	}
	return result
}
