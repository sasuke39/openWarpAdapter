package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/sasuke39/openWarpAdapter/internal/agent"
	"github.com/sasuke39/openWarpAdapter/internal/config"
	"github.com/sasuke39/openWarpAdapter/internal/llm"

	"github.com/openai/openai-go"
	pb "github.com/sasuke39/openWarpAdapter/internal/proto"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Conversation struct {
	mu                       sync.Mutex
	history                  []openai.ChatCompletionMessageParamUnion
	client                   *llm.Client
	CreatedAt                time.Time
	LastRequestID            string
	LastRunID                string
	LastLongRunningCommandID string
}

type Server struct {
	mu              sync.RWMutex
	conversations   map[string]*Conversation
	runningTasks    sync.Map // taskID → context.CancelFunc
	cfg             *config.Config
	persistencePath string
}

const maxConversations = 30

var supportedMVPTools = map[string]struct{}{
	"read_files":                             {},
	"grep":                                   {},
	"file_glob":                              {},
	"file_glob_v2":                           {},
	"run_shell_command":                      {},
	"read_shell_command_output":              {},
	"transfer_shell_command_control_to_user": {},
	"apply_file_diffs":                       {},
	"search_codebase":                        {},
}

func NewServer(cfg *config.Config, configPath string) *Server {
	server := &Server{
		conversations:   make(map[string]*Conversation),
		cfg:             cfg,
		persistencePath: filepath.Join(filepath.Dir(configPath), "conversations.json"),
	}
	if err := server.loadConversations(); err != nil {
		log.Printf("Failed to load persisted conversations: %v", err)
	}
	return server
}

func (s *Server) getOrCreateConversation(id string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conv, ok := s.conversations[id]; ok {
		return conv
	}
	conv := &Conversation{
		client:    llm.NewClient(s.cfg),
		CreatedAt: time.Now().UTC(),
	}
	s.conversations[id] = conv
	s.evictOldestLocked()
	return conv
}

func (s *Server) evictOldestLocked() {
	for len(s.conversations) > maxConversations {
		var oldestID string
		var oldestTime time.Time
		first := true
		for id, conv := range s.conversations {
			createdAt := conv.CreatedAt
			if createdAt.IsZero() {
				createdAt = time.Unix(0, 0).UTC()
			}
			if first || createdAt.Before(oldestTime) {
				first = false
				oldestID = id
				oldestTime = createdAt
			}
		}
		if oldestID == "" {
			return
		}
		delete(s.conversations, oldestID)
		log.Printf("Evicted oldest conversation %s to enforce limit=%d", oldestID, maxConversations)
	}
}

func main() {
	configPath := "config.yaml"
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server := NewServer(cfg, configPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/multi-agent", server.handleAgentRequest)
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/signup/remote", server.handleSignupRemote)
	mux.HandleFunc("/login/remote", server.handleSignupRemote)
	mux.HandleFunc("POST /agent/tasks/{task_id}/cancel", server.handleCancelTask)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		if err := server.saveConversations(); err != nil {
			log.Printf("Failed to persist conversations on shutdown: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	log.Printf("Local adapter listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}

func (s *Server) handleSignupRemote(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	scheme := r.URL.Query().Get("scheme")
	if scheme == "" {
		scheme = "warplocal"
	}
	redirectURL := fmt.Sprintf("%s://auth/desktop_redirect?refresh_token=local&state=%s", scheme, state)

	log.Printf("[/signup/remote] scheme=%s state=%s → redirecting to %s", scheme, state, redirectURL)

	// Return an HTML page that redirects via JavaScript.
	// Browsers may block 302 redirects to custom URL schemes, but allow
	// user-initiated or JS-triggered navigations.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Warp Local</title></head>
<body>
<p>Logging in to local Warp adapter...</p>
<p>If nothing happens, <a href="%s">click here</a>.</p>
<script>window.location.href = "%s";</script>
</body>
</html>`, redirectURL, redirectURL)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}
	if cancel, ok := s.runningTasks.Load(taskID); ok {
		if fn, ok := cancel.(context.CancelFunc); ok {
			fn()
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAgentRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[REQ] Failed to read body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	log.Printf("[REQ] body_size=%d", len(body))

	var req pb.Request
	if err := proto.Unmarshal(body, &req); err != nil {
		log.Printf("[REQ] Failed to unmarshal: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	convID := req.GetMetadata().GetConversationId()
	if convID == "" {
		convID = uuid.New().String()
	}

	// Extract task ID from the request — must match what the client sent
	// in TaskContext.tasks, otherwise AddMessagesToTask won't find the task.
	taskID := "task-" + uuid.New().String()
	taskIDFromClient := false
	if tc := req.GetTaskContext(); tc != nil {
		if tasks := tc.GetTasks(); len(tasks) > 0 {
			taskID = tasks[0].GetId()
			taskIDFromClient = true
			log.Printf("[REQ] using task_id from request: %s (found %d tasks)", taskID, len(tasks))
		} else {
			log.Printf("[REQ] WARNING: TaskContext has no tasks, using generated task_id=%s", taskID)
		}
	} else {
		log.Printf("[REQ] WARNING: no TaskContext in request, using generated task_id=%s", taskID)
	}

	conv := s.getOrCreateConversation(convID)

	// Extract user inputs from request
	inputs := extractInputs(&req)
	isFollowUp := len(inputs) > 0 && inputs[0].Kind == "tool_result"

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	conv.mu.Lock()
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = time.Now().UTC()
	}
	requestID := uuid.New().String()
	runID := uuid.New().String()
	if isFollowUp && conv.LastRequestID != "" && conv.LastRunID != "" {
		requestID = conv.LastRequestID
		runID = conv.LastRunID
	} else {
		conv.LastRequestID = requestID
		conv.LastRunID = runID
	}

	log.Printf("[REQ] conv=%s req=%s run=%s history_len=%d", convID, requestID, runID, len(conv.history))

	// Send StreamInit
	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_Init{
			Init: &pb.ResponseEvent_StreamInit{
				ConversationId: convID,
				RequestId:      requestID,
				RunId:          runID,
			},
		},
	})

	if len(inputs) == 0 {
		conv.mu.Unlock()
		log.Printf("[REQ] No inputs found in request, sending empty finish")
		s.sendEvent(w, flusher, finishEvent(&pb.ResponseEvent_StreamFinished_Done{}))
		return
	}

	for i, in := range inputs {
		contentPreview := in.Content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "..."
		}
		log.Printf("[REQ] input[%d] kind=%s tool_call_id=%s content=%q", i, in.Kind, in.ToolCallID, contentPreview)
	}

	// Feed inputs into conversation history
	for _, in := range inputs {
		if in.LongRunningCommandID != "" {
			conv.LastLongRunningCommandID = in.LongRunningCommandID
		}
		if in.ShellCommandCompleted {
			conv.LastLongRunningCommandID = ""
		}
		switch in.Kind {
		case "user_query":
			conv.history = append(conv.history, llm.MakeUserMessage(in.Content))
		case "tool_result":
			conv.history = append(conv.history, llm.MakeToolResultMessage(in.ToolCallID, in.Content))
		}
	}

	log.Printf("[REQ] history now has %d messages, calling LLM", len(conv.history))

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Register the cancel function so the /agent/tasks/{task_id}/cancel
	// endpoint can stop a running agent loop.
	s.runningTasks.Store(taskID, cancel)
	defer s.runningTasks.Delete(taskID)

	s.runAgentLoop(ctx, w, flusher, conv, requestID, taskID, isFollowUp || taskIDFromClient)
	conv.mu.Unlock()
	if err := s.saveConversations(); err != nil {
		log.Printf("Failed to persist conversations: %v", err)
	}
}

type input struct {
	Kind                  string // "user_query" or "tool_result"
	Content               string
	ToolCallID            string
	LongRunningCommandID  string
	ShellCommandCompleted bool
}

func extractInputs(req *pb.Request) []input {
	var inputs []input

	switch v := req.GetInput().GetType().(type) {
	case *pb.Request_Input_UserQuery_:
		inputs = append(inputs, input{
			Kind:    "user_query",
			Content: v.UserQuery.GetQuery(),
		})
	case *pb.Request_Input_UserInputs_:
		for _, u := range v.UserInputs.GetInputs() {
			switch ui := u.GetInput().(type) {
			case *pb.Request_Input_UserInputs_UserInput_UserQuery:
				inputs = append(inputs, input{
					Kind:    "user_query",
					Content: ui.UserQuery.GetQuery(),
				})
			case *pb.Request_Input_UserInputs_UserInput_ToolCallResult:
				inputs = append(inputs, extractToolResult(ui.ToolCallResult))
			}
		}
	}
	return inputs
}

func extractToolResult(tc *pb.Request_Input_ToolCallResult) input {
	ui := input{
		Kind:       "tool_result",
		ToolCallID: tc.GetToolCallId(),
	}
	if result := tc.GetRunShellCommand(); result != nil {
		if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
			ui.LongRunningCommandID = snapshot.GetCommandId()
		}
		if result.GetCommandFinished() != nil {
			ui.ShellCommandCompleted = true
		}
	}
	if result := tc.GetReadShellCommandOutput(); result != nil {
		if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
			ui.LongRunningCommandID = snapshot.GetCommandId()
		}
		if result.GetCommandFinished() != nil {
			ui.ShellCommandCompleted = true
		}
	}
	if result := tc.GetTransferShellCommandControlToUser(); result != nil {
		if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
			ui.LongRunningCommandID = snapshot.GetCommandId()
		}
		if result.GetCommandFinished() != nil {
			ui.ShellCommandCompleted = true
		}
	}
	ui.Content = summarizeToolCallResult(tc)
	return ui
}

type persistedConversation struct {
	History                  []json.RawMessage `json:"history"`
	LastRequestID            string            `json:"last_request_id"`
	LastRunID                string            `json:"last_run_id"`
	LastLongRunningCommandID string            `json:"last_long_running_command_id,omitempty"`
	CreatedAt                time.Time         `json:"created_at"`
}

type storedMessage struct {
	Role             string                    `json:"role"`
	Content          string                    `json:"content,omitempty"`
	ReasoningContent string                    `json:"reasoning_content,omitempty"`
	ToolCallID       string                    `json:"tool_call_id,omitempty"`
	ToolCalls        []storedAssistantToolCall `json:"tool_calls,omitempty"`
}

type storedAssistantToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (s *Server) loadConversations() error {
	data, err := os.ReadFile(s.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var persisted map[string]persistedConversation
	if err := json.Unmarshal(data, &persisted); err != nil {
		return err
	}

	for id, item := range persisted {
		history, err := deserializeHistory(item.History)
		if err != nil {
			log.Printf("Skipping persisted conversation %s: %v", id, err)
			continue
		}
		createdAt := item.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		s.conversations[id] = &Conversation{
			history:                  history,
			client:                   llm.NewClient(s.cfg),
			CreatedAt:                createdAt,
			LastRequestID:            item.LastRequestID,
			LastRunID:                item.LastRunID,
			LastLongRunningCommandID: item.LastLongRunningCommandID,
		}
	}

	s.evictOldestLocked()
	log.Printf("Loaded %d persisted conversations from %s", len(s.conversations), s.persistencePath)
	return nil
}

func (s *Server) saveConversations() error {
	s.mu.RLock()
	ids := make([]string, 0, len(s.conversations))
	for id := range s.conversations {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	persisted := make(map[string]persistedConversation, len(ids))
	for _, id := range ids {
		conv := s.conversations[id]
		conv.mu.Lock()
		history, err := serializeHistory(conv.history)
		if err != nil {
			conv.mu.Unlock()
			s.mu.RUnlock()
			return fmt.Errorf("serialize conversation %s: %w", id, err)
		}
		createdAt := conv.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		persisted[id] = persistedConversation{
			History:                  history,
			LastRequestID:            conv.LastRequestID,
			LastRunID:                conv.LastRunID,
			LastLongRunningCommandID: conv.LastLongRunningCommandID,
			CreatedAt:                createdAt,
		}
		conv.mu.Unlock()
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.persistencePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.persistencePath, data, 0o644)
}

func serializeHistory(history []openai.ChatCompletionMessageParamUnion) ([]json.RawMessage, error) {
	items := make([]json.RawMessage, 0, len(history))
	for _, msg := range history {
		b, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		items = append(items, json.RawMessage(b))
	}
	return items, nil
}

func deserializeHistory(rawMessages []json.RawMessage) ([]openai.ChatCompletionMessageParamUnion, error) {
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(rawMessages))
	for _, raw := range rawMessages {
		var msg storedMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}

		switch msg.Role {
		case "user":
			history = append(history, llm.MakeUserMessage(msg.Content))
		case "tool":
			history = append(history, llm.MakeToolResultMessage(msg.ToolCallID, msg.Content))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]llm.ToolCall, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					toolCalls = append(toolCalls, llm.ToolCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
						Args: json.RawMessage(tc.Function.Arguments),
					})
				}
				history = append(history, llm.MakeAssistantToolCallMessage(toolCalls, msg.ReasoningContent))
				continue
			}
			history = append(history, llm.MakeAssistantMessageWithReasoning(msg.Content, msg.ReasoningContent))
		default:
			log.Printf("Skipping unsupported persisted message role=%q", msg.Role)
		}
	}
	return history, nil
}

func summarizeToolCallResult(tc *pb.Request_Input_ToolCallResult) string {
	switch {
	case tc.GetRunShellCommand() != nil:
		return summarizeRunShellCommandResult(tc.GetRunShellCommand())
	case tc.GetReadFiles() != nil:
		return summarizeReadFilesResult(tc.GetReadFiles())
	case tc.GetSearchCodebase() != nil:
		return summarizeSearchCodebaseResult(tc.GetSearchCodebase())
	case tc.GetApplyFileDiffs() != nil:
		return summarizeApplyFileDiffsResult(tc.GetApplyFileDiffs())
	case tc.GetGrep() != nil:
		return summarizeGrepResult(tc.GetGrep())
	case tc.GetFileGlob() != nil:
		return summarizeFileGlobResult(tc.GetFileGlob())
	case tc.GetFileGlobV2() != nil:
		return summarizeFileGlobV2Result(tc.GetFileGlobV2())
	case tc.GetReadShellCommandOutput() != nil:
		return summarizeReadShellCommandOutputResult(tc.GetReadShellCommandOutput())
	case tc.GetTransferShellCommandControlToUser() != nil:
		return summarizeTransferShellCommandControlToUserResult(tc.GetTransferShellCommandControlToUser())
	default:
		if b, err := json.Marshal(tc.GetResult()); err == nil {
			return string(b)
		}
		return "tool result received"
	}
}

func summarizeRunShellCommandResult(result *pb.RunShellCommandResult) string {
	if finished := result.GetCommandFinished(); finished != nil {
		output := strings.TrimSpace(finished.GetOutput())
		if output == "" {
			output = "(no output)"
		}
		return fmt.Sprintf("Command: %s\nExit Code: %d\nOutput:\n%s", result.GetCommand(), finished.GetExitCode(), output)
	}
	if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
		output := strings.TrimSpace(snapshot.GetOutput())
		if output == "" {
			output = "(no output yet)"
		}
		return fmt.Sprintf("Command still running: %s\nCommand ID: %s\nCurrent Output:\n%s", result.GetCommand(), snapshot.GetCommandId(), output)
	}
	if denied := result.GetPermissionDenied(); denied != nil {
		return fmt.Sprintf("Command denied: %s\nReason: %s", result.GetCommand(), summarizePermissionDenied(denied))
	}
	if result.GetOutput() != "" || result.GetExitCode() != 0 {
		return fmt.Sprintf("Command: %s\nExit Code: %d\nOutput:\n%s", result.GetCommand(), result.GetExitCode(), strings.TrimSpace(result.GetOutput()))
	}
	return fmt.Sprintf("Command finished: %s", result.GetCommand())
}

func summarizePermissionDenied(denied *pb.PermissionDenied) string {
	switch denied.GetReason().(type) {
	case *pb.PermissionDenied_DenylistedCommand:
		return "command is denylisted"
	default:
		return "permission denied"
	}
}

func summarizeReadFilesResult(result *pb.ReadFilesResult) string {
	if success := result.GetTextFilesSuccess(); success != nil {
		return joinFileContents(success.GetFiles())
	}
	if success := result.GetAnyFilesSuccess(); success != nil {
		var sections []string
		for _, file := range success.GetFiles() {
			if text := file.GetTextContent(); text != nil {
				sections = append(sections, formatFileContent(text))
				continue
			}
			if binary := file.GetBinaryContent(); binary != nil {
				sections = append(sections, fmt.Sprintf("File: %s\n<binary content: %d bytes>", binary.GetFilePath(), len(binary.GetData())))
			}
		}
		if len(sections) == 0 {
			return "Read files succeeded with no readable content."
		}
		return strings.Join(sections, "\n\n")
	}
	if errResult := result.GetError(); errResult != nil {
		return "Read files failed: " + errResult.GetMessage()
	}
	return "Read files completed."
}

func summarizeSearchCodebaseResult(result *pb.SearchCodebaseResult) string {
	if success := result.GetSuccess(); success != nil {
		return joinFileContents(success.GetFiles())
	}
	if errResult := result.GetError(); errResult != nil {
		return "Search codebase failed: " + errResult.GetMessage()
	}
	return "Search codebase completed."
}

func summarizeApplyFileDiffsResult(result *pb.ApplyFileDiffsResult) string {
	if success := result.GetSuccess(); success != nil {
		var parts []string
		for _, file := range success.GetUpdatedFilesV2() {
			section := formatFileContent(file.GetFile())
			if file.GetWasEditedByUser() {
				section += "\nNote: file includes user edits."
			}
			parts = append(parts, section)
		}
		for _, file := range success.GetUpdatedFiles() {
			parts = append(parts, formatFileContent(file))
		}
		for _, deleted := range success.GetDeletedFiles() {
			parts = append(parts, fmt.Sprintf("Deleted file: %s", deleted.GetFilePath()))
		}
		if len(parts) == 0 {
			return "Apply file diffs succeeded with no file details."
		}
		return strings.Join(parts, "\n\n")
	}
	if errResult := result.GetError(); errResult != nil {
		return "Apply file diffs failed: " + errResult.GetMessage()
	}
	return "Apply file diffs completed."
}

func summarizeGrepResult(result *pb.GrepResult) string {
	if success := result.GetSuccess(); success != nil {
		var parts []string
		for _, file := range success.GetMatchedFiles() {
			lines := make([]string, 0, len(file.GetMatchedLines()))
			for _, line := range file.GetMatchedLines() {
				lines = append(lines, fmt.Sprintf("%d", line.GetLineNumber()))
			}
			parts = append(parts, fmt.Sprintf("%s: lines %s", file.GetFilePath(), strings.Join(lines, ", ")))
		}
		if len(parts) == 0 {
			return "Grep succeeded with no matches."
		}
		return strings.Join(parts, "\n")
	}
	if errResult := result.GetError(); errResult != nil {
		return "Grep failed: " + errResult.GetMessage()
	}
	return "Grep completed."
}

func summarizeFileGlobResult(result *pb.FileGlobResult) string {
	if success := result.GetSuccess(); success != nil {
		matches := strings.TrimSpace(success.GetMatchedFiles())
		if matches == "" {
			return "File glob succeeded with no matches."
		}
		return "Matched files:\n" + matches
	}
	if errResult := result.GetError(); errResult != nil {
		return "File glob failed: " + errResult.GetMessage()
	}
	return "File glob completed."
}

func summarizeFileGlobV2Result(result *pb.FileGlobV2Result) string {
	if success := result.GetSuccess(); success != nil {
		lines := make([]string, 0, len(success.GetMatchedFiles()))
		for _, match := range success.GetMatchedFiles() {
			lines = append(lines, match.GetFilePath())
		}
		if warnings := strings.TrimSpace(success.GetWarnings()); warnings != "" {
			lines = append(lines, "Warnings: "+warnings)
		}
		if len(lines) == 0 {
			return "File glob v2 succeeded with no matches."
		}
		return "Matched files:\n" + strings.Join(lines, "\n")
	}
	if errResult := result.GetError(); errResult != nil {
		return "File glob v2 failed: " + errResult.GetMessage()
	}
	return "File glob v2 completed."
}

func summarizeReadShellCommandOutputResult(result *pb.ReadShellCommandOutputResult) string {
	if finished := result.GetCommandFinished(); finished != nil {
		output := strings.TrimSpace(finished.GetOutput())
		if output == "" {
			output = "(no output)"
		}
		return fmt.Sprintf("Command finished.\nExit Code: %d\nOutput:\n%s", finished.GetExitCode(), output)
	}
	if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
		output := strings.TrimSpace(snapshot.GetOutput())
		if output == "" {
			output = "(no output yet)"
		}
		return fmt.Sprintf("Command still running.\nCommand ID: %s\nCurrent Output:\n%s", snapshot.GetCommandId(), output)
	}
	if errResult := result.GetError(); errResult != nil {
		return "Reading shell command output failed: " + summarizeShellCommandError(errResult)
	}
	return "Shell command output fetched."
}

func summarizeTransferShellCommandControlToUserResult(result *pb.TransferShellCommandControlToUserResult) string {
	if finished := result.GetCommandFinished(); finished != nil {
		output := strings.TrimSpace(finished.GetOutput())
		if output == "" {
			output = "(no output)"
		}
		return fmt.Sprintf("Command finished after handing control to user.\nExit Code: %d\nOutput:\n%s", finished.GetExitCode(), output)
	}
	if snapshot := result.GetLongRunningCommandSnapshot(); snapshot != nil {
		output := strings.TrimSpace(snapshot.GetOutput())
		if output == "" {
			output = "(no output yet)"
		}
		return fmt.Sprintf("Command handed off to user.\nCommand ID: %s\nCurrent Output:\n%s", snapshot.GetCommandId(), output)
	}
	if errResult := result.GetError(); errResult != nil {
		return "Transfer shell command control failed: " + summarizeShellCommandError(errResult)
	}
	return "Shell command control transferred to user."
}

func summarizeShellCommandError(errResult *pb.ShellCommandError) string {
	switch errResult.GetType().(type) {
	case *pb.ShellCommandError_CommandNotFound:
		return "command not found"
	default:
		return "unknown shell command error"
	}
}

func joinFileContents(files []*pb.FileContent) string {
	if len(files) == 0 {
		return "No file content returned."
	}
	sections := make([]string, 0, len(files))
	for _, file := range files {
		sections = append(sections, formatFileContent(file))
	}
	return strings.Join(sections, "\n\n")
}

func formatFileContent(file *pb.FileContent) string {
	if file == nil {
		return "<missing file content>"
	}
	header := "File: " + file.GetFilePath()
	if lineRange := file.GetLineRange(); lineRange != nil {
		header = fmt.Sprintf("%s (lines %d-%d)", header, lineRange.GetStart(), lineRange.GetEnd())
	}
	content := strings.TrimSpace(file.GetContent())
	if content == "" {
		content = "(empty)"
	}
	return header + "\n" + content
}

func (s *Server) runAgentLoop(ctx context.Context, w io.Writer, flusher http.Flusher, conv *Conversation, requestID, taskID string, isFollowUp bool) {
	// Only send CreateTask on the first request — it upgrades the client's
	// optimistic task. On follow-up requests (tool results), the task already exists.
	if !isFollowUp {
		s.sendCreateTask(w, flusher, taskID)
	}

	const maxLoops = 5
	for i := 0; i < maxLoops; i++ {
		// Check if client disconnected before starting a new loop iteration.
		if ctx.Err() != nil {
			log.Printf("[LLM] loop=%d context cancelled before stream: %v", i, ctx.Err())
			return
		}

		log.Printf("[LLM] loop=%d starting stream, task_id=%s history_len=%d", i, taskID, len(conv.history))
		stream := conv.client.StreamChat(ctx, agent.SystemPrompt, conv.history)

		var chunks []openai.ChatCompletionChunk
		chunkCount := 0
		textChars := 0

		// Fixed message ID so AppendToMessageContent can target the same message
		outputMsgID := uuid.New().String()
		var firstSent bool

		for stream.Next() {
			chunk := stream.Current()
			chunks = append(chunks, chunk)
			chunkCount++

			for _, choice := range chunk.Choices {
				// Debug: log raw delta JSON on early chunks to inspect reasoning_content
				if chunkCount <= 3 {
					rawDelta := choice.Delta.RawJSON()
					if len(rawDelta) > 500 {
						rawDelta = rawDelta[:500] + "..."
					}
					log.Printf("[LLM] loop=%d chunk=%d delta_raw=%s", i, chunkCount, rawDelta)
				}
				if choice.Delta.Content != "" {
					textChars += len(choice.Delta.Content)
					if !firstSent {
						s.sendFirstTextChunk(w, flusher, taskID, requestID, outputMsgID, choice.Delta.Content)
						firstSent = true
					} else {
						s.sendAppendText(w, flusher, taskID, outputMsgID, choice.Delta.Content)
					}
				}
			}
		}

		log.Printf("[LLM] loop=%d stream done: chunks=%d text_chars=%d", i, chunkCount, textChars)

		if err := stream.Err(); err != nil {
			log.Printf("[LLM] loop=%d STREAM ERROR: %v", i, err)
			// If the client disconnected, don't try to write an error event —
			// the connection is already closed.
			if ctx.Err() != nil {
				log.Printf("[LLM] loop=%d context also cancelled, skipping error event", i)
				return
			}
			s.sendFinishError(w, flusher, err.Error())
			return
		}

		result := llm.CollectStreamResult(chunks)
		rcPreview := result.ReasoningContent
		if len(rcPreview) > 200 {
			rcPreview = rcPreview[:200] + "..."
		}
		log.Printf("[LLM] loop=%d result: text_len=%d reasoning_len=%d is_tool=%v reasoning_preview=%q", i, len(result.Text), len(result.ReasoningContent), result.IsToolCall, rcPreview)

		if len(chunks) > 0 {
			last := chunks[len(chunks)-1]
			for _, choice := range last.Choices {
				log.Printf("[LLM] loop=%d finish_reason=%q content_len=%d", i, choice.FinishReason, len(choice.Delta.Content))
			}
		}

		// Check if LLM wants to call tools
		if result.IsToolCall {
			for j, tc := range result.ToolCalls {
				log.Printf("[LLM] loop=%d tool_call[%d] name=%s id=%s args=%s", i, j, tc.Name, tc.ID, string(tc.Args))
			}
			if err := validateMVPToolCalls(result.ToolCalls); err != nil {
				log.Printf("[LLM] loop=%d unsupported tool call: %v", i, err)
				s.sendFinishError(w, flusher, err.Error())
				return
			}
			conv.history = append(conv.history, llm.MakeAssistantToolCallMessage(result.ToolCalls, result.ReasoningContent))
			if err := s.sendToolCalls(w, flusher, conv, taskID, result.ToolCalls); err != nil {
				log.Printf("[LLM] loop=%d failed to send tool calls: %v", i, err)
				s.sendFinishError(w, flusher, err.Error())
				return
			}
			s.sendEvent(w, flusher, finishEvent(&pb.ResponseEvent_StreamFinished_Done{}))
			return
		}

		textPreview := result.Text
		if len(textPreview) > 300 {
			textPreview = textPreview[:300] + "..."
		}
		log.Printf("[LLM] loop=%d final_text len=%d preview=%q", i, len(result.Text), textPreview)

		if result.Text == "" {
			log.Printf("[LLM] loop=%d empty response, sending error", i)
			s.sendFinishError(w, flusher, "LLM returned empty response")
			return
		}

		conv.history = append(conv.history, llm.MakeAssistantMessageWithReasoning(result.Text, result.ReasoningContent))

		log.Printf("[LLM] loop=%d sending Done finish event", i)
		s.sendEvent(w, flusher, finishEvent(&pb.ResponseEvent_StreamFinished_Done{}))
		return
	}

	log.Printf("[LLM] Max tool loops reached")
	s.sendFinishError(w, flusher, "Max tool call loops exceeded")
}

func (s *Server) sendCreateTask(w io.Writer, flusher http.Flusher, taskID string) {
	log.Printf("[LLM] sending CreateTask for task_id=%s", taskID)
	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_ClientActions_{
			ClientActions: &pb.ResponseEvent_ClientActions{
				Actions: []*pb.ClientAction{
					{
						Action: &pb.ClientAction_CreateTask_{
							CreateTask: &pb.ClientAction_CreateTask{
								Task: &pb.Task{Id: taskID},
							},
						},
					},
				},
			},
		},
	})
}

// sendFirstTextChunk sends the first text delta via AddMessagesToTask, creating
// the message that subsequent AppendToMessageContent calls will append to.
func (s *Server) sendFirstTextChunk(w io.Writer, flusher http.Flusher, taskID, requestID, msgID, delta string) {
	msg := &pb.Message{
		Id:        msgID,
		TaskId:    taskID,
		RequestId: requestID,
		Timestamp: timestamppb.Now(),
		Message: &pb.Message_AgentOutput_{
			AgentOutput: &pb.Message_AgentOutput{
				Text: delta,
			},
		},
	}
	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_ClientActions_{
			ClientActions: &pb.ResponseEvent_ClientActions{
				Actions: []*pb.ClientAction{
					{
						Action: &pb.ClientAction_AddMessagesToTask_{
							AddMessagesToTask: &pb.ClientAction_AddMessagesToTask{
								TaskId:   taskID,
								Messages: []*pb.Message{msg},
							},
						},
					},
				},
			},
		},
	})
}

// sendAppendText appends a text delta to an existing AgentOutput message.
func (s *Server) sendAppendText(w io.Writer, flusher http.Flusher, taskID, msgID, delta string) {
	msg := &pb.Message{
		Id: msgID,
		Message: &pb.Message_AgentOutput_{
			AgentOutput: &pb.Message_AgentOutput{
				Text: delta,
			},
		},
	}
	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_ClientActions_{
			ClientActions: &pb.ResponseEvent_ClientActions{
				Actions: []*pb.ClientAction{
					{
						Action: &pb.ClientAction_AppendToMessageContent_{
							AppendToMessageContent: &pb.ClientAction_AppendToMessageContent{
								TaskId:  taskID,
								Message: msg,
								Mask: &fieldmaskpb.FieldMask{
									Paths: []string{"agent_output.text"},
								},
							},
						},
					},
				},
			},
		},
	})
}

func (s *Server) sendFinishError(w io.Writer, flusher http.Flusher, message string) {
	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_Finished{
			Finished: &pb.ResponseEvent_StreamFinished{
				Reason: &pb.ResponseEvent_StreamFinished_InternalError_{
					InternalError: &pb.ResponseEvent_StreamFinished_InternalError{
						Message: message,
					},
				},
			},
		},
	})
}

func (s *Server) sendIncrementalText(w io.Writer, flusher http.Flusher, taskID, requestID, delta string) {
	msg := &pb.Message{
		Id:        uuid.New().String(),
		TaskId:    taskID,
		RequestId: requestID,
		Timestamp: timestamppb.Now(),
		Message: &pb.Message_AgentOutput_{
			AgentOutput: &pb.Message_AgentOutput{
				Text: delta,
			},
		},
	}

	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_ClientActions_{
			ClientActions: &pb.ResponseEvent_ClientActions{
				Actions: []*pb.ClientAction{
					{
						Action: &pb.ClientAction_AddMessagesToTask_{
							AddMessagesToTask: &pb.ClientAction_AddMessagesToTask{
								TaskId:   taskID,
								Messages: []*pb.Message{msg},
							},
						},
					},
				},
			},
		},
	})
}

func validateMVPToolCalls(toolCalls []llm.ToolCall) error {
	for _, tc := range toolCalls {
		if _, ok := supportedMVPTools[tc.Name]; !ok {
			return fmt.Errorf("tool %s is not supported by local adapter MVP", tc.Name)
		}
	}
	return nil
}

func (s *Server) sendToolCalls(w io.Writer, flusher http.Flusher, conv *Conversation, taskID string, toolCalls []llm.ToolCall) error {
	msgs := make([]*pb.Message, 0, len(toolCalls))
	for _, tc := range toolCalls {
		tcMsg := &pb.Message_ToolCall{
			ToolCallId: tc.ID,
		}
		// Build tool variant inline since isMessage_ToolCall_Tool is unexported.
		switch tc.Name {
		case "read_files":
			var args struct {
				Files []struct {
					Name       string `json:"name"`
					LineRanges []struct {
						Start int `json:"start"`
						End   int `json:"end"`
					} `json:"line_ranges"`
				} `json:"files"`
			}
			json.Unmarshal(tc.Args, &args)
			files := make([]*pb.Message_ToolCall_ReadFiles_File, 0, len(args.Files))
			for _, f := range args.Files {
				ranges := make([]*pb.FileContentLineRange, 0, len(f.LineRanges))
				for _, lr := range f.LineRanges {
					ranges = append(ranges, &pb.FileContentLineRange{
						Start: uint32(lr.Start),
						End:   uint32(lr.End),
					})
				}
				files = append(files, &pb.Message_ToolCall_ReadFiles_File{
					Name:       f.Name,
					LineRanges: ranges,
				})
			}
			tcMsg.Tool = &pb.Message_ToolCall_ReadFiles_{
				ReadFiles: &pb.Message_ToolCall_ReadFiles{Files: files},
			}

		case "grep":
			var args struct {
				Queries []string `json:"queries"`
				Path    string   `json:"path"`
			}
			json.Unmarshal(tc.Args, &args)
			tcMsg.Tool = &pb.Message_ToolCall_Grep_{
				Grep: &pb.Message_ToolCall_Grep{
					Queries: args.Queries,
					Path:    args.Path,
				},
			}

		case "file_glob":
			var args struct {
				Patterns  []string `json:"patterns"`
				Path      string   `json:"path"`
				SearchDir string   `json:"search_dir"`
			}
			json.Unmarshal(tc.Args, &args)
			path := args.Path
			if path == "" {
				path = args.SearchDir
			}
			tcMsg.Tool = &pb.Message_ToolCall_FileGlob_{
				FileGlob: &pb.Message_ToolCall_FileGlob{
					Patterns: args.Patterns,
					Path:     path,
				},
			}

		case "file_glob_v2":
			var args struct {
				Patterns   []string `json:"patterns"`
				SearchDir  string   `json:"search_dir"`
				MaxMatches int32    `json:"max_matches"`
				MaxDepth   int32    `json:"max_depth"`
				MinDepth   int32    `json:"min_depth"`
			}
			json.Unmarshal(tc.Args, &args)
			tcMsg.Tool = &pb.Message_ToolCall_FileGlobV2_{
				FileGlobV2: &pb.Message_ToolCall_FileGlobV2{
					Patterns:   args.Patterns,
					SearchDir:  args.SearchDir,
					MaxMatches: args.MaxMatches,
					MaxDepth:   args.MaxDepth,
					MinDepth:   args.MinDepth,
				},
			}

		case "run_shell_command":
			var args struct {
				Command      string `json:"command"`
				IsReadOnly   bool   `json:"is_read_only"`
				IsRisky      bool   `json:"is_risky"`
				RiskCategory string `json:"risk_category"`
			}
			json.Unmarshal(tc.Args, &args)
			if strings.TrimSpace(args.Command) == "wait" && conv.LastLongRunningCommandID != "" {
				tcMsg.Tool = &pb.Message_ToolCall_ReadShellCommandOutput_{
					ReadShellCommandOutput: &pb.Message_ToolCall_ReadShellCommandOutput{
						CommandId: conv.LastLongRunningCommandID,
						Delay: &pb.Message_ToolCall_ReadShellCommandOutput_OnCompletion{
							OnCompletion: &emptypb.Empty{},
						},
					},
				}
				break
			}
			tcMsg.Tool = &pb.Message_ToolCall_RunShellCommand_{
				RunShellCommand: &pb.Message_ToolCall_RunShellCommand{
					Command:      args.Command,
					IsReadOnly:   args.IsReadOnly,
					IsRisky:      args.IsRisky,
					RiskCategory: parseRiskCategory(args.RiskCategory),
				},
			}

		case "read_shell_command_output":
			var args struct {
				CommandID         string `json:"command_id"`
				WaitForCompletion *bool  `json:"wait_for_completion"`
				DurationSeconds   *int64 `json:"duration_seconds"`
			}
			json.Unmarshal(tc.Args, &args)
			commandID := args.CommandID
			if commandID == "" {
				commandID = conv.LastLongRunningCommandID
			}
			read := &pb.Message_ToolCall_ReadShellCommandOutput{
				CommandId: commandID,
			}
			if args.WaitForCompletion == nil || *args.WaitForCompletion {
				read.Delay = &pb.Message_ToolCall_ReadShellCommandOutput_OnCompletion{
					OnCompletion: &emptypb.Empty{},
				}
			} else {
				seconds := int64(1)
				if args.DurationSeconds != nil && *args.DurationSeconds > 0 {
					seconds = *args.DurationSeconds
				}
				read.Delay = &pb.Message_ToolCall_ReadShellCommandOutput_Duration{
					Duration: durationpb.New(time.Duration(seconds) * time.Second),
				}
			}
			tcMsg.Tool = &pb.Message_ToolCall_ReadShellCommandOutput_{
				ReadShellCommandOutput: read,
			}

		case "transfer_shell_command_control_to_user":
			var args struct {
				Reason string `json:"reason"`
			}
			json.Unmarshal(tc.Args, &args)
			tcMsg.Tool = &pb.Message_ToolCall_TransferShellCommandControlToUser_{
				TransferShellCommandControlToUser: &pb.Message_ToolCall_TransferShellCommandControlToUser{
					Reason: args.Reason,
				},
			}

		case "apply_file_diffs":
			var args struct {
				Summary string `json:"summary"`
				Diffs   []struct {
					FilePath string `json:"file_path"`
					Search   string `json:"search"`
					Replace  string `json:"replace"`
				} `json:"diffs"`
				NewFiles []struct {
					FilePath string `json:"file_path"`
					Content  string `json:"content"`
				} `json:"new_files"`
				DeletedFiles []struct {
					FilePath string `json:"file_path"`
				} `json:"deleted_files"`
			}
			json.Unmarshal(tc.Args, &args)
			pbDiffs := make([]*pb.Message_ToolCall_ApplyFileDiffs_FileDiff, 0, len(args.Diffs))
			for _, d := range args.Diffs {
				pbDiffs = append(pbDiffs, &pb.Message_ToolCall_ApplyFileDiffs_FileDiff{
					FilePath: d.FilePath,
					Search:   d.Search,
					Replace:  d.Replace,
				})
			}
			pbNewFiles := make([]*pb.Message_ToolCall_ApplyFileDiffs_NewFile, 0, len(args.NewFiles))
			for _, nf := range args.NewFiles {
				pbNewFiles = append(pbNewFiles, &pb.Message_ToolCall_ApplyFileDiffs_NewFile{
					FilePath: nf.FilePath,
					Content:  nf.Content,
				})
			}
			pbDeleted := make([]*pb.Message_ToolCall_ApplyFileDiffs_DeleteFile, 0, len(args.DeletedFiles))
			for _, df := range args.DeletedFiles {
				pbDeleted = append(pbDeleted, &pb.Message_ToolCall_ApplyFileDiffs_DeleteFile{
					FilePath: df.FilePath,
				})
			}
			tcMsg.Tool = &pb.Message_ToolCall_ApplyFileDiffs_{
				ApplyFileDiffs: &pb.Message_ToolCall_ApplyFileDiffs{
					Summary:      args.Summary,
					Diffs:        pbDiffs,
					NewFiles:     pbNewFiles,
					DeletedFiles: pbDeleted,
				},
			}

		case "search_codebase":
			var args struct {
				Query        string   `json:"query"`
				PathFilters  []string `json:"path_filters"`
				CodebasePath string   `json:"codebase_path"`
			}
			json.Unmarshal(tc.Args, &args)
			tcMsg.Tool = &pb.Message_ToolCall_SearchCodebase_{
				SearchCodebase: &pb.Message_ToolCall_SearchCodebase{
					Query:        args.Query,
					PathFilters:  args.PathFilters,
					CodebasePath: args.CodebasePath,
				},
			}
		default:
			return fmt.Errorf("tool %s is not supported by local adapter MVP", tc.Name)
		}

		msg := &pb.Message{
			Id:        uuid.New().String(),
			TaskId:    taskID,
			Timestamp: timestamppb.Now(),
			Message: &pb.Message_ToolCall_{
				ToolCall: tcMsg,
			},
		}
		msgs = append(msgs, msg)
	}

	s.sendEvent(w, flusher, &pb.ResponseEvent{
		Type: &pb.ResponseEvent_ClientActions_{
			ClientActions: &pb.ResponseEvent_ClientActions{
				Actions: []*pb.ClientAction{
					{
						Action: &pb.ClientAction_AddMessagesToTask_{
							AddMessagesToTask: &pb.ClientAction_AddMessagesToTask{
								TaskId:   taskID,
								Messages: msgs,
							},
						},
					},
				},
			},
		},
	})
	return nil
}

func parseRiskCategory(s string) pb.RiskCategory {
	switch s {
	case "RISK_CATEGORY_READ_ONLY":
		return pb.RiskCategory_RISK_CATEGORY_READ_ONLY
	case "RISK_CATEGORY_TRIVIAL_LOCAL_CHANGE":
		return pb.RiskCategory_RISK_CATEGORY_TRIVIAL_LOCAL_CHANGE
	case "RISK_CATEGORY_NONTRIVIAL_LOCAL_CHANGE":
		return pb.RiskCategory_RISK_CATEGORY_NONTRIVIAL_LOCAL_CHANGE
	case "RISK_CATEGORY_EXTERNAL_CHANGE":
		return pb.RiskCategory_RISK_CATEGORY_EXTERNAL_CHANGE
	case "RISK_CATEGORY_RISKY":
		return pb.RiskCategory_RISK_CATEGORY_RISKY
	default:
		return pb.RiskCategory_RISK_CATEGORY_UNSPECIFIED
	}
}

func finishEvent(done *pb.ResponseEvent_StreamFinished_Done) *pb.ResponseEvent {
	return &pb.ResponseEvent{
		Type: &pb.ResponseEvent_Finished{
			Finished: &pb.ResponseEvent_StreamFinished{
				Reason: &pb.ResponseEvent_StreamFinished_Done_{
					Done: done,
				},
			},
		},
	}
}

func (s *Server) sendEvent(w io.Writer, flusher http.Flusher, event *pb.ResponseEvent) {
	data, err := proto.Marshal(event)
	if err != nil {
		log.Printf("[EVENT] Failed to marshal event: %v", err)
		return
	}
	encoded := base64.URLEncoding.EncodeToString(data)
	fmt.Fprintf(w, "data: %s\n\n", encoded)
	flusher.Flush()

	// Log event type for debugging
	switch event.Type.(type) {
	case *pb.ResponseEvent_Init:
		init := event.GetInit()
		log.Printf("[EVENT] StreamInit conv=%s req=%s run=%s", init.GetConversationId(), init.GetRequestId(), init.GetRunId())
	case *pb.ResponseEvent_ClientActions_:
		actions := event.GetClientActions()
		for _, a := range actions.GetActions() {
			switch a.Action.(type) {
			case *pb.ClientAction_AddMessagesToTask_:
				amt := a.GetAddMessagesToTask()
				for _, m := range amt.GetMessages() {
					switch m.Message.(type) {
					case *pb.Message_AgentOutput_:
						log.Printf("[EVENT] ClientAction AddMessagesToTask AgentOutput task=%s len=%d", amt.GetTaskId(), len(m.GetAgentOutput().GetText()))
					case *pb.Message_ToolCall_:
						tc := m.GetToolCall()
						log.Printf("[EVENT] ClientAction AddMessagesToTask ToolCall task=%s tool=%T", amt.GetTaskId(), tc.GetTool())
					default:
						log.Printf("[EVENT] ClientAction AddMessagesToTask task=%s msg_type=%T", amt.GetTaskId(), m.Message)
					}
				}
			default:
				log.Printf("[EVENT] ClientAction type=%T", a.Action)
			}
		}
	case *pb.ResponseEvent_Finished:
		fin := event.GetFinished()
		log.Printf("[EVENT] StreamFinished reason=%T", fin.GetReason())
	default:
		log.Printf("[EVENT] unknown type=%T", event.Type)
	}
}

var _ = json.RawMessage{}
