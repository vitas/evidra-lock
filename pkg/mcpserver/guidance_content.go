package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	evidra "samebits.com/evidra"
)

const (
	contentFileInitializeInstructions  = "initialize/instructions.txt"
	contentFileValidateToolDescription = "tools/validate_description.txt"
	contentFileGetEventToolDescription = "tools/get_event_description.txt"

	contentFileDocsEngineLogicBody = "resources/content/docs_engine_logic_v2.md"
	contentFileDocsProtocolBody    = "resources/content/protocol_errors.md"
	contentFileAgentContractBody   = "resources/content/agent_contract_v1.md"

	contentFileDocsEngineLogicDescription = "resources/descriptions/docs_engine_logic_v2.txt"
	contentFileDocsProtocolDescription    = "resources/descriptions/protocol_errors.txt"
	contentFilePolicySummaryDescription   = "resources/descriptions/policy_summary.txt"
	contentFileAgentContractDescription   = "resources/descriptions/agent_contract_v1.txt"
)

var defaultContentDirRelative = filepath.Join("prompts", "mcpserver")
var embeddedContentDir = path.Join("prompts", "mcpserver")
var errGuidanceContentDirNotFound = errors.New("guidance content directory not found")

type GuidanceContent struct {
	InitializeInstructions        string
	ValidateToolDescription       string
	GetEventToolDescription       string
	DocsEngineLogicV2Body         string
	DocsProtocolErrorsBody        string
	AgentContractV1Body           string
	DocsEngineLogicV2Description  string
	DocsProtocolErrorsDescription string
	PolicySummaryDescription      string
	AgentContractDescription      string
}

func mustLoadGuidanceContent(explicitDir string) GuidanceContent {
	content, err := loadGuidanceContentAuto(explicitDir)
	if err != nil {
		panic(err)
	}
	return content
}

func loadGuidanceContentAuto(explicitDir string) (GuidanceContent, error) {
	dir, err := resolveGuidanceContentDir(explicitDir)
	if err == nil {
		return loadGuidanceContent(dir)
	}
	if !errors.Is(err, errGuidanceContentDirNotFound) {
		return GuidanceContent{}, err
	}
	return loadEmbeddedGuidanceContent()
}

func loadGuidanceContent(baseDir string) (GuidanceContent, error) {
	read := func(rel string) (string, error) {
		raw, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("read guidance file %q: %w", rel, err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return loadGuidanceContentFromRead(read)
}

func loadEmbeddedGuidanceContent() (GuidanceContent, error) {
	read := func(rel string) (string, error) {
		target := path.Join(embeddedContentDir, rel)
		raw, err := fs.ReadFile(evidra.MCPServerContentFS, target)
		if err != nil {
			return "", fmt.Errorf("read embedded guidance file %q: %w", rel, err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return loadGuidanceContentFromRead(read)
}

func loadGuidanceContentFromRead(read func(rel string) (string, error)) (GuidanceContent, error) {
	initialize, err := read(contentFileInitializeInstructions)
	if err != nil {
		return GuidanceContent{}, err
	}
	validateDesc, err := read(contentFileValidateToolDescription)
	if err != nil {
		return GuidanceContent{}, err
	}
	getEventDesc, err := read(contentFileGetEventToolDescription)
	if err != nil {
		return GuidanceContent{}, err
	}
	engineBody, err := read(contentFileDocsEngineLogicBody)
	if err != nil {
		return GuidanceContent{}, err
	}
	protocolBody, err := read(contentFileDocsProtocolBody)
	if err != nil {
		return GuidanceContent{}, err
	}
	contractBody, err := read(contentFileAgentContractBody)
	if err != nil {
		return GuidanceContent{}, err
	}
	engineDesc, err := read(contentFileDocsEngineLogicDescription)
	if err != nil {
		return GuidanceContent{}, err
	}
	protocolDesc, err := read(contentFileDocsProtocolDescription)
	if err != nil {
		return GuidanceContent{}, err
	}
	policyDesc, err := read(contentFilePolicySummaryDescription)
	if err != nil {
		return GuidanceContent{}, err
	}
	contractDesc, err := read(contentFileAgentContractDescription)
	if err != nil {
		return GuidanceContent{}, err
	}

	return GuidanceContent{
		InitializeInstructions:        initialize,
		ValidateToolDescription:       validateDesc,
		GetEventToolDescription:       getEventDesc,
		DocsEngineLogicV2Body:         engineBody,
		DocsProtocolErrorsBody:        protocolBody,
		AgentContractV1Body:           contractBody,
		DocsEngineLogicV2Description:  engineDesc,
		DocsProtocolErrorsDescription: protocolDesc,
		PolicySummaryDescription:      policyDesc,
		AgentContractDescription:      contractDesc,
	}, nil
}

func resolveGuidanceContentDir(explicitDir string) (string, error) {
	if dir := strings.TrimSpace(explicitDir); dir != "" {
		validated, err := validateGuidanceContentDir(dir)
		if err != nil {
			return "", fmt.Errorf("resolve guidance content dir from --content-dir: %w", err)
		}
		return validated, nil
	}
	if dir := strings.TrimSpace(os.Getenv("EVIDRA_CONTENT_DIR")); dir != "" {
		validated, err := validateGuidanceContentDir(dir)
		if err != nil {
			return "", fmt.Errorf("resolve guidance content dir from EVIDRA_CONTENT_DIR: %w", err)
		}
		return validated, nil
	}

	if cwd, err := os.Getwd(); err == nil {
		if dir, ok := findGuidanceContentDir(cwd); ok {
			return dir, nil
		}
	}

	if exePath, err := os.Executable(); err == nil {
		if dir, ok := findGuidanceContentDir(filepath.Dir(exePath)); ok {
			return dir, nil
		}
	}

	return "", errGuidanceContentDirNotFound
}

func findGuidanceContentDir(startDir string) (string, bool) {
	dir := filepath.Clean(startDir)
	for {
		candidate := filepath.Join(dir, defaultContentDirRelative)
		if validated, err := validateGuidanceContentDir(candidate); err == nil {
			return validated, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func validateGuidanceContentDir(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve guidance content dir %q: %w", dir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("guidance content path is not a directory: %s", absDir)
	}
	for _, rel := range []string{
		contentFileInitializeInstructions,
		contentFileValidateToolDescription,
		contentFileGetEventToolDescription,
		contentFileDocsEngineLogicBody,
		contentFileDocsProtocolBody,
		contentFileAgentContractBody,
		contentFileDocsEngineLogicDescription,
		contentFileDocsProtocolDescription,
		contentFilePolicySummaryDescription,
		contentFileAgentContractDescription,
	} {
		target := filepath.Join(absDir, filepath.FromSlash(rel))
		fileInfo, statErr := os.Stat(target)
		if statErr != nil {
			return "", fmt.Errorf("guidance content missing %q in %s: %w", rel, absDir, statErr)
		}
		if fileInfo.IsDir() {
			return "", fmt.Errorf("guidance content file expected, got directory: %s", target)
		}
	}
	return absDir, nil
}
