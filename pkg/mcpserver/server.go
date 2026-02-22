package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/core"
	"samebits.com/evidra-mcp/pkg/engine"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/outputlimit"
	"samebits.com/evidra-mcp/pkg/registry"
)

type Options struct {
	Name                     string
	Version                  string
	Mode                     Mode
	Guarded                  bool
	PolicyRef                string
	EvidencePath             string
	IncludeFileResourceLinks bool
	MaxOutputBytes           int
}

type Mode = engine.Mode

const (
	ModeEnforce = engine.ModeEnforce
	ModeObserve = engine.ModeObserve
)

type ExecuteOutput struct {
	OK        bool             `json:"ok"`
	EventID   string           `json:"event_id,omitempty"`
	Policy    PolicySummary    `json:"policy"`
	Execution ExecutionSummary `json:"execution"`
	Resources []ResourceLink   `json:"resources,omitempty"`
	Hints     []string         `json:"hints,omitempty"`
	Error     *ErrorSummary    `json:"error,omitempty"`
}

type PolicySummary struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	PolicyRef string `json:"policy_ref"`
}

type ExecutionSummary struct {
	Status          string `json:"status"`
	ExitCode        *int   `json:"exit_code"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}

type ErrorSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RiskLevel string `json:"risk_level,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

type GetEventOutput struct {
	OK        bool             `json:"ok"`
	Record    *evidence.Record `json:"record,omitempty"`
	Resources []ResourceLink   `json:"resources,omitempty"`
	Error     *ErrorSummary    `json:"error,omitempty"`
}

type ResourceLink struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MIMEType string `json:"mimeType,omitempty"`
}

type executeHandler struct {
	service *ExecuteService
}

type getEventHandler struct {
	service *ExecuteService
}

type getEventInput struct {
	EventID string `json:"event_id"`
}

func NewServer(opts Options, reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}
	if opts.Mode == "" {
		opts.Mode = ModeEnforce
	}
	if opts.EvidencePath == "" {
		opts.EvidencePath = "./data/evidence"
	}
	if opts.MaxOutputBytes <= 0 {
		opts.MaxOutputBytes = outputlimit.DefaultMaxBytes
	}

	svc := newExecuteService(reg, policyEngine, evidenceStore, opts.Mode, opts.PolicyRef, opts.Guarded, opts.MaxOutputBytes, nil)
	svc.evidencePath = opts.EvidencePath
	svc.includeFileResourceLinks = opts.IncludeFileResourceLinks
	executeTool := &executeHandler{service: svc}
	getEventTool := &getEventHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Title:       "Execute Tool Invocation",
		Description: "Execute a canonical ToolInvocation through Registry -> Policy -> Execution -> Evidence. Typical use: controlled ops actions with event_id evidence linkage.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Controlled Execution",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(true),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"actor", "tool", "operation", "params", "context"},
			"properties": map[string]any{
				"actor": map[string]any{
					"type":        "object",
					"description": "Execution initiator identity.",
					"required":    []string{"type", "id", "origin"},
					"properties": map[string]any{
						"type":   map[string]any{"type": "string", "description": "Actor category (human|ai|system)."},
						"id":     map[string]any{"type": "string", "description": "Actor identifier."},
						"origin": map[string]any{"type": "string", "description": "Invocation source (mcp|cli|api|unknown)."},
					},
				},
				"tool":      map[string]any{"type": "string", "description": "Registered tool name (required). Example: terraform, argocd, helm."},
				"operation": map[string]any{"type": "string", "description": "Registered operation for tool (required). Example: plan, app-sync, upgrade."},
				"params":    map[string]any{"type": "object", "description": "Operation parameters; validated by tool schema."},
				"context":   map[string]any{"type": "object", "description": "Execution context for policy decisions. Example: {\"environment\":\"dev\"}."},
			},
		},
	}, executeTool.Handle)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Title:       "Get Evidence Event",
		Description: "Fetch one immutable evidence record by event_id. Typical use: inspect policy decision and execution details after execute.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Evidence Lookup",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"event_id"},
			"properties": map[string]any{
				"event_id": map[string]any{"type": "string", "description": "Evidence event identifier from execute response (required). Example: evt-1234567890."},
			},
		},
	}, getEventTool.Handle)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "evidra-event",
		Title:       "Evidence Event Record",
		Description: "Read a specific evidence record by event_id.",
		MIMEType:    "application/json",
		URITemplate: "evidra://event/{event_id}",
	}, svc.readResourceEvent)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-evidence-manifest",
		Title:       "Evidence Manifest",
		Description: "Read evidence manifest for segmented store.",
		MIMEType:    "application/json",
		URI:         "evidra://evidence/manifest",
	}, svc.readResourceManifest)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-evidence-segments",
		Title:       "Evidence Segments",
		Description: "Read sealed/current segment summary.",
		MIMEType:    "application/json",
		URI:         "evidra://evidence/segments",
	}, svc.readResourceSegments)
	return server
}

func (h *executeHandler) Handle(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ExecuteOutput, error) {
	reporter := progressReporterFromRequest(ctx, req)
	output := h.service.ExecuteWithReporter(ctx, input, reporter)
	return &mcp.CallToolResult{
		Content: resourceLinksToContent(output.Resources),
	}, output, nil
}

func (h *getEventHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getEventInput,
) (*mcp.CallToolResult, GetEventOutput, error) {
	output := h.service.GetEvent(ctx, input.EventID)
	return &mcp.CallToolResult{
		Content: resourceLinksToContent(output.Resources),
	}, output, nil
}

type ExecuteService struct {
	exec                     *engine.ExecutionEngine
	policyRef                string
	evidencePath             string
	includeFileResourceLinks bool
}

func NewExecuteService(reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) *ExecuteService {
	return NewExecuteServiceWithMode(reg, policyEngine, evidenceStore, ModeEnforce, "")
}

func NewExecuteServiceWithMode(reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore, mode Mode, policyRef string) *ExecuteService {
	return newExecuteService(reg, policyEngine, evidenceStore, mode, policyRef, false, outputlimit.DefaultMaxBytes, nil)
}

func newExecuteService(reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore, mode Mode, policyRef string, guarded bool, maxOutputBytes int, runner engine.Runner) *ExecuteService {
	exec := engine.NewExecutionEngine(registry.NewEngineToolResolver(reg), policyEngine, evidenceStore, engine.Config{
		Mode:           mode,
		Guarded:        guarded,
		PolicyRef:      policyRef,
		MaxOutputBytes: maxOutputBytes,
		Runner:         runner,
	})
	return &ExecuteService{
		exec:         exec,
		policyRef:    policyRef,
		evidencePath: "./data/evidence",
	}
}

func (s *ExecuteService) GetEvent(_ context.Context, eventID string) GetEventOutput {
	if eventID == "" {
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "invalid_input",
				Message: "event_id is required",
			},
		}
	}

	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return GetEventOutput{
				OK: false,
				Error: &ErrorSummary{
					Code:    "evidence_chain_invalid",
					Message: "evidence chain validation failed",
				},
			}
		}
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "internal_error",
				Message: "failed to read evidence",
			},
		}
	}
	if !found {
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "not_found",
				Message: "event_id not found",
			},
		}
	}
	return GetEventOutput{
		OK:        true,
		Record:    &rec,
		Resources: s.resourceLinks(rec.EventID),
	}
}

func (s *ExecuteService) Execute(ctx context.Context, inv invocation.ToolInvocation) ExecuteOutput {
	return s.ExecuteWithReporter(ctx, inv, nil)
}

type ProgressReporter func(progress float64, message string)

func (s *ExecuteService) ExecuteWithReporter(ctx context.Context, inv invocation.ToolInvocation, reporter ProgressReporter) ExecuteOutput {
	var rep engine.Reporter
	if reporter != nil {
		rep = engine.ReporterFunc(reporter)
	}
	res, err := s.exec.Execute(ctx, inv, rep)
	if err != nil {
		return ExecuteOutput{
			OK: false,
			Policy: PolicySummary{
				Allow:     false,
				RiskLevel: "critical",
				Reason:    "internal_error",
				PolicyRef: s.policyRef,
			},
			Execution: ExecutionSummary{
				Status: "failed",
			},
			Error: &ErrorSummary{
				Code:    "internal_error",
				Message: "execution pipeline failed",
			},
		}
	}
	out := ExecuteOutput{
		OK:      res.OK,
		EventID: res.EvidenceID,
		Policy: PolicySummary{
			Allow:     res.Decision.Allow,
			RiskLevel: res.Decision.RiskLevel,
			Reason:    res.Decision.Reason,
			PolicyRef: s.policyRef,
		},
		Execution: ExecutionSummary{
			Status:          res.Output.Status,
			ExitCode:        res.Output.ExitCode,
			Stdout:          res.Output.Stdout,
			Stderr:          res.Output.Stderr,
			StdoutTruncated: res.Output.StdoutTruncated,
			StderrTruncated: res.Output.StderrTruncated,
		},
		Resources: s.resourceLinks(res.EvidenceID),
		Hints:     res.Hints,
	}
	if res.Error != nil {
		out.Error = &ErrorSummary{
			Code:      res.Error.Code,
			Message:   res.Error.Message,
			RiskLevel: res.Error.RiskLevel,
			Reason:    res.Error.Reason,
			Hint:      res.Error.Hint,
		}
	}
	return out
}

func boolPtr(v bool) *bool {
	return &v
}

func progressReporterFromRequest(ctx context.Context, req *mcp.CallToolRequest) ProgressReporter {
	if req == nil || req.Session == nil {
		return nil
	}
	progressToken := req.Params.GetProgressToken()
	if progressToken == nil {
		return nil
	}
	return func(progress float64, message string) {
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: progressToken,
			Progress:      progress,
			Total:         100,
			Message:       message,
		})
	}
}

func (s *ExecuteService) resourceLinks(eventID string) []ResourceLink {
	links := []ResourceLink{
		{
			URI:      fmt.Sprintf("evidra://event/%s", eventID),
			Name:     "Evidence record",
			MIMEType: "application/json",
		},
	}
	mode, resolved, err := evidenceStorePathInfo(s.evidencePath)
	if err != nil {
		return links
	}
	if mode == "segmented" {
		links = append(links,
			ResourceLink{URI: "evidra://evidence/manifest", Name: "Evidence manifest", MIMEType: "application/json"},
			ResourceLink{URI: "evidra://evidence/segments", Name: "Evidence segments", MIMEType: "application/json"},
		)
	}
	if s.includeFileResourceLinks {
		abs, absErr := filepath.Abs(resolved)
		if absErr == nil {
			links = append(links, ResourceLink{
				URI:      "file://" + filepath.ToSlash(abs),
				Name:     "Local evidence path",
				MIMEType: "text/plain",
			})
		}
	}
	return links
}

func resourceLinksToContent(links []ResourceLink) []mcp.Content {
	if len(links) == 0 {
		return nil
	}
	out := make([]mcp.Content, 0, len(links))
	for _, l := range links {
		out = append(out, &mcp.ResourceLink{
			URI:      l.URI,
			Name:     l.Name,
			MIMEType: l.MIMEType,
		})
	}
	return out
}

func evidenceStorePathInfo(path string) (mode string, resolved string, err error) {
	mode, err = evidence.StoreFormatAtPath(path)
	if err != nil {
		return "", "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		resolved = path
	} else {
		resolved = abs
	}
	return mode, resolved, nil
}

func (s *ExecuteService) readResourceEvent(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	eventID := strings.TrimPrefix(req.Params.URI, "evidra://event/")
	if eventID == "" || eventID == req.Params.URI {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	if !found {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(b),
			},
		},
	}, nil
}

func (s *ExecuteService) readResourceManifest(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "evidra://evidence/manifest",
				MIMEType: "application/json",
				Text:     string(b),
			},
		},
	}, nil
}

func (s *ExecuteService) readResourceSegments(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	payload := map[string]any{
		"sealed_segments": m.SealedSegments,
		"current_segment": m.CurrentSegment,
		"count":           len(m.SealedSegments),
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      "evidra://evidence/segments",
				MIMEType: "application/json",
				Text:     string(b),
			},
		},
	}, nil
}
