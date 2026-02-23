package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/config"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/validate"
)

type Mode string

const (
	ModeEnforce Mode = "enforce"
	ModeObserve Mode = "observe"
)

type Options struct {
	Name                     string
	Version                  string
	Mode                     Mode
	PolicyRef                string
	PolicyPath               string
	DataPath                 string
	EvidencePath             string
	IncludeFileResourceLinks bool
}

type PolicySummary struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	PolicyRef string `json:"policy_ref,omitempty"`
}

type ErrorSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RiskLevel string `json:"risk_level,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

type ValidateOutput struct {
	OK        bool           `json:"ok"`
	EventID   string         `json:"event_id,omitempty"`
	Policy    PolicySummary  `json:"policy"`
	RuleIDs   []string               `json:"rule_ids,omitempty"`
	Hints     []string               `json:"hints,omitempty"`
	Reasons   []string               `json:"reasons,omitempty"`
	Resources []evidence.ResourceLink `json:"resources,omitempty"`
	Error     *ErrorSummary  `json:"error,omitempty"`
}

type validateHandler struct {
	service *ValidateService
}

type getEventHandler struct {
	service *ValidateService
}

type getEventInput struct {
	EventID string `json:"event_id"`
}

func NewServer(opts Options) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}
	if opts.Mode != ModeObserve {
		opts.Mode = ModeEnforce
	}
	if opts.EvidencePath == "" {
		opts.EvidencePath = config.DefaultEvidenceDir
	}

	svc := newValidateService(opts)
	validateTool := &validateHandler{service: svc}
	getEventTool := &getEventHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate",
		Title:       "Validate Tool Invocation",
		Description: "Run the validation scenario without executing commands. Provide tool/operation metadata and risk tags to inspect policy hits, hints, and evidence.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validate Scenario",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"actor", "tool", "operation", "params", "context"},
			"properties": map[string]any{
				"actor": map[string]any{
					"type":        "object",
					"description": "Invocation initiator identity.",
					"required":    []any{"type", "id", "origin"},
					"properties": map[string]any{
						"type":   map[string]any{"type": "string", "description": "Actor category (human|agent|system)."},
						"id":     map[string]any{"type": "string", "description": "Actor identifier."},
						"origin": map[string]any{"type": "string", "description": "Invocation source (mcp|cli|api)."},
					},
				},
				"tool":      map[string]any{"type": "string", "description": "Tool name (e.g. terraform)."},
				"operation": map[string]any{"type": "string", "description": "Operation (e.g. plan, apply)."},
				"params":    map[string]any{"type": "object", "description": "Operation parameters; include risk_tags/asserted data."},
				"context":   map[string]any{"type": "object", "description": "Optional context metadata."},
			},
		},
	}, validateTool.Handle)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Title:       "Get Evidence Event",
		Description: "Fetch one immutable evidence record by event_id.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Evidence Lookup",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"event_id"},
			"properties": map[string]any{
				"event_id": map[string]any{"type": "string", "description": "Evidence event identifier."},
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

func (h *validateHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ValidateOutput, error) {
	output := h.service.Validate(ctx, input)
	return &mcp.CallToolResult{Content: resourceLinksToContent(output.Resources)}, output, nil
}

func (h *getEventHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getEventInput,
) (*mcp.CallToolResult, GetEventOutput, error) {
	output := h.service.GetEvent(ctx, input.EventID)
	return &mcp.CallToolResult{Content: resourceLinksToContent(output.Resources)}, output, nil
}

func newValidateService(opts Options) *ValidateService {
	return &ValidateService{
		policyPath:               opts.PolicyPath,
		dataPath:                 opts.DataPath,
		evidencePath:             opts.EvidencePath,
		policyRef:                opts.PolicyRef,
		mode:                     opts.Mode,
		includeFileResourceLinks: opts.IncludeFileResourceLinks,
	}
}

type ValidateService struct {
	policyPath               string
	dataPath                 string
	evidencePath             string
	policyRef                string
	mode                     Mode
	includeFileResourceLinks bool
}

type GetEventOutput struct {
	OK        bool             `json:"ok"`
	Record    *evidence.Record `json:"record,omitempty"`
	Resources []evidence.ResourceLink `json:"resources,omitempty"`
	Error     *ErrorSummary    `json:"error,omitempty"`
}

func (s *ValidateService) Validate(ctx context.Context, inv invocation.ToolInvocation) ValidateOutput {
	if err := inv.ValidateStructure(); err != nil {
		return ValidateOutput{
			OK: false,
			Policy: PolicySummary{
				Allow:     false,
				RiskLevel: "critical",
				Reason:    "invalid_input",
				PolicyRef: s.policyRef,
			},
			Error: &ErrorSummary{
				Code:    "invalid_input",
				Message: err.Error(),
			},
		}
	}

	res, err := validate.EvaluateInvocation(ctx, inv, validate.Options{
		PolicyPath:  s.policyPath,
		DataPath:    s.dataPath,
		EvidenceDir: s.evidencePath,
	})
	if err != nil {
		return ValidateOutput{
			OK: false,
			Policy: PolicySummary{
				Allow:     false,
				RiskLevel: "critical",
				Reason:    "internal_error",
				PolicyRef: s.policyRef,
			},
			Error: &ErrorSummary{
				Code:    "internal_error",
				Message: "validation pipeline failed",
			},
		}
	}

	ok := res.Pass
	if s.mode == ModeObserve {
		ok = true
	}

	return ValidateOutput{
		OK:      ok,
		EventID: res.EvidenceID,
		Policy: PolicySummary{
			Allow:     res.Pass,
			RiskLevel: res.RiskLevel,
			Reason:    firstReason(res.Reasons),
			PolicyRef: s.policyRef,
		},
		RuleIDs:   res.RuleIDs,
		Hints:     res.Hints,
		Reasons:   res.Reasons,
		Resources: s.resourceLinks(res.EvidenceID),
	}
}

func firstReason(reasons []string) string {
	if len(reasons) == 0 {
		return "scenario_validated"
	}
	return reasons[0]
}

func (s *ValidateService) GetEvent(_ context.Context, eventID string) GetEventOutput {
	if eventID == "" {
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: "invalid_input", Message: "event_id is required"}}
	}
	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return GetEventOutput{OK: false, Error: &ErrorSummary{Code: "evidence_chain_invalid", Message: "evidence chain validation failed"}}
		}
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: "internal_error", Message: "failed to read evidence"}}
	}
	if !found {
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: "not_found", Message: "event_id not found"}}
	}
	return GetEventOutput{OK: true, Record: &rec, Resources: s.resourceLinks(rec.EventID)}
}

func (s *ValidateService) resourceLinks(eventID string) []evidence.ResourceLink {
	return evidence.ResourceLinks(eventID, s.evidencePath, s.includeFileResourceLinks)
}

func resourceLinksToContent(links []evidence.ResourceLink) []mcp.Content {
	if len(links) == 0 {
		return nil
	}
	out := make([]mcp.Content, 0, len(links))
	for _, l := range links {
		out = append(out, &mcp.ResourceLink{URI: l.URI, Name: l.Name, MIMEType: l.MIMEType})
	}
	return out
}

func (s *ValidateService) readResourceEvent(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	eventID := strings.TrimPrefix(req.Params.URI, "evidra://event/")
	if eventID == "" || eventID == req.Params.URI {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil || !found {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceManifest(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "evidra://evidence/manifest", MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceSegments(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	payload := map[string]any{"sealed_segments": m.SealedSegments, "current_segment": m.CurrentSegment, "count": len(m.SealedSegments)}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "evidra://evidence/segments", MIMEType: "application/json", Text: string(b)}}}, nil
}

func boolPtr(v bool) *bool {
	return &v
}
