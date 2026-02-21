package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reflow/gateway/internal/mcp"
	"github.com/rs/zerolog/log"
)

// Process wraps an exec.Cmd running an MCP server via STDIO transport.
// It implements mcp.MCPClient.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu           sync.RWMutex
	initialized  bool
	capabilities *mcp.ServerCapabilities
	serverInfo   *mcp.ServerInfo

	pending  map[string]chan *mcp.JSONRPCResponse
	pendMu   sync.Mutex
	nextID   int64
	lastUsed atomic.Int64
	done     chan struct{}

	subjectKey string
	targetName string
}

// ProcessConfig holds configuration for creating a STDIO process.
type ProcessConfig struct {
	Command    string
	Args       []string
	Env        []string
	SubjectKey string
	TargetName string
}

// NewProcess creates and starts a new STDIO MCP process.
func NewProcess(cfg ProcessConfig) (*Process, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = cfg.Env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command %q: %w", cfg.Command, err)
	}

	p := &Process{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		pending:    make(map[string]chan *mcp.JSONRPCResponse),
		done:       make(chan struct{}),
		subjectKey: cfg.SubjectKey,
		targetName: cfg.TargetName,
	}
	p.lastUsed.Store(time.Now().Unix())

	go p.readLoop()
	go p.stderrLoop()

	log.Info().
		Str("target", cfg.TargetName).
		Str("subject_key", cfg.SubjectKey).
		Str("command", cfg.Command).
		Int("pid", cmd.Process.Pid).
		Msg("Started STDIO MCP process")

	return p, nil
}

// readLoop reads line-delimited JSON from stdout and routes responses.
func (p *Process) readLoop() {
	defer close(p.done)
	scanner := bufio.NewScanner(p.stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Debug().
				Str("target", p.targetName).
				Str("line", string(line)).
				Err(err).
				Msg("Failed to parse STDIO response")
			continue
		}

		if resp.ID != nil {
			reqID := string(resp.ID)
			p.pendMu.Lock()
			if ch, ok := p.pending[reqID]; ok {
				ch <- &resp
				delete(p.pending, reqID)
			}
			p.pendMu.Unlock()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Debug().Err(err).Str("target", p.targetName).Msg("STDIO stdout reader ended")
	}
}

// stderrLoop logs stderr output from the process.
func (p *Process) stderrLoop() {
	scanner := bufio.NewScanner(p.stderr)
	for scanner.Scan() {
		log.Debug().
			Str("target", p.targetName).
			Str("subject_key", p.subjectKey).
			Str("stderr", scanner.Text()).
			Msg("STDIO process stderr")
	}
}

// allocID returns a unique JSON-RPC request ID.
func (p *Process) allocID() json.RawMessage {
	id := atomic.AddInt64(&p.nextID, 1)
	return json.RawMessage(fmt.Sprintf("%d", id))
}

// sendRequest writes a JSON-RPC request to stdin and waits for the response.
func (p *Process) sendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	p.lastUsed.Store(time.Now().Unix())

	reqID := string(req.ID)
	responseCh := make(chan *mcp.JSONRPCResponse, 1)

	p.pendMu.Lock()
	p.pending[reqID] = responseCh
	p.pendMu.Unlock()

	defer func() {
		p.pendMu.Lock()
		delete(p.pending, reqID)
		p.pendMu.Unlock()
	}()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := p.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	select {
	case resp := <-responseCh:
		return resp, nil
	case <-p.done:
		return nil, fmt.Errorf("STDIO process exited while waiting for response")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("timeout waiting for STDIO response")
	}
}

// sendNotification writes a JSON-RPC notification to stdin (no response expected).
func (p *Process) sendNotification(req *mcp.JSONRPCNotification) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	data = append(data, '\n')
	_, err = p.stdin.Write(data)
	return err
}

// Initialize sends the initialize request to the STDIO process.
func (p *Process) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodInitialize,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal initialize result: %w", err)
	}

	p.mu.Lock()
	p.initialized = true
	p.capabilities = &result.Capabilities
	p.serverInfo = &result.ServerInfo
	p.mu.Unlock()

	// Send initialized notification
	notification := &mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPCVersion,
		Method:  mcp.MethodInitialized,
	}
	if err := p.sendNotification(notification); err != nil {
		log.Warn().Err(err).Msg("Failed to send initialized notification to STDIO process")
	}

	return &result, nil
}

// ListTools retrieves tools from the STDIO process.
func (p *Process) ListTools(ctx context.Context, cursor *string) (*mcp.ToolsListResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodToolsList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tools/list: %w", err)
	}
	return &result, nil
}

// CallTool calls a tool on the STDIO process.
func (p *Process) CallTool(ctx context.Context, params *mcp.ToolCallParams) (*mcp.ToolCallResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodToolsCall,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.ToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tools/call: %w", err)
	}
	return &result, nil
}

// ListResources retrieves resources from the STDIO process.
func (p *Process) ListResources(ctx context.Context, cursor *string) (*mcp.ResourcesListResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodResourcesList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.ResourcesListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal resources/list: %w", err)
	}
	return &result, nil
}

// ReadResource reads a resource from the STDIO process.
func (p *Process) ReadResource(ctx context.Context, uri string) (*mcp.ResourceReadResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodResourcesRead,
	}
	params := mcp.ResourceReadParams{URI: uri}
	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("resources/read error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.ResourceReadResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal resources/read: %w", err)
	}
	return &result, nil
}

// ListPrompts retrieves prompts from the STDIO process.
func (p *Process) ListPrompts(ctx context.Context, cursor *string) (*mcp.PromptsListResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodPromptsList,
	}
	if cursor != nil {
		params := map[string]string{"cursor": *cursor}
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.PromptsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal prompts/list: %w", err)
	}
	return &result, nil
}

// GetPrompt retrieves a prompt from the STDIO process.
func (p *Process) GetPrompt(ctx context.Context, params *mcp.PromptGetParams) (*mcp.PromptGetResult, error) {
	req := &mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      p.allocID(),
		Method:  mcp.MethodPromptsGet,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	req.Params = paramsJSON

	resp, err := p.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/get error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result mcp.PromptGetResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal prompts/get: %w", err)
	}
	return &result, nil
}

// SendRawRequest sends a raw JSON-RPC request to the STDIO process.
func (p *Process) SendRawRequest(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	return p.sendRequest(ctx, req)
}

// IsInitialized returns whether the process has been initialized.
func (p *Process) IsInitialized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}

// GetCapabilities returns the server capabilities.
func (p *Process) GetCapabilities() *mcp.ServerCapabilities {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.capabilities
}

// GetServerInfo returns the server info.
func (p *Process) GetServerInfo() *mcp.ServerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.serverInfo
}

// IsAlive checks if the process is still running.
func (p *Process) IsAlive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// LastUsed returns the last time the process was used.
func (p *Process) LastUsed() time.Time {
	return time.Unix(p.lastUsed.Load(), 0)
}

// PID returns the process ID, or 0 if not running.
func (p *Process) PID() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// Close gracefully stops the process.
func (p *Process) Close() error {
	log.Info().
		Str("target", p.targetName).
		Str("subject_key", p.subjectKey).
		Msg("Stopping STDIO MCP process")

	// Close stdin to signal the process to exit
	p.stdin.Close()

	// Wait up to 5 seconds for graceful exit
	waitCh := make(chan error, 1)
	go func() { waitCh <- p.cmd.Wait() }()

	select {
	case <-waitCh:
		return nil
	case <-time.After(5 * time.Second):
		log.Warn().Str("target", p.targetName).Msg("STDIO process did not exit gracefully, killing")
		if p.cmd.Process != nil {
			return p.cmd.Process.Kill()
		}
		return nil
	}
}
