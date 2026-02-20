package evidence

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra-mcp/pkg/invocation"
)

type PolicyDecision struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	Advisory  bool   `json:"advisory"`
}

type ExecutionResult struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

type EvidenceRecord struct {
	EventID         string                 `json:"event_id"`
	Timestamp       time.Time              `json:"timestamp"`
	PolicyRef       string                 `json:"policy_ref"`
	Actor           invocation.Actor       `json:"actor"`
	Tool            string                 `json:"tool"`
	Operation       string                 `json:"operation"`
	Params          map[string]interface{} `json:"params"`
	PolicyDecision  PolicyDecision         `json:"policy_decision"`
	ExecutionResult ExecutionResult        `json:"execution_result"`
	PreviousHash    string                 `json:"previous_hash"`
	Hash            string                 `json:"hash"`
}

type canonicalEvidenceRecord struct {
	EventID         string                 `json:"event_id"`
	Timestamp       time.Time              `json:"timestamp"`
	PolicyRef       string                 `json:"policy_ref"`
	Actor           invocation.Actor       `json:"actor"`
	Tool            string                 `json:"tool"`
	Operation       string                 `json:"operation"`
	Params          map[string]interface{} `json:"params"`
	PolicyDecision  PolicyDecision         `json:"policy_decision"`
	ExecutionResult ExecutionResult        `json:"execution_result"`
	PreviousHash    string                 `json:"previous_hash"`
}

type StoreManifest struct {
	Format          string `json:"format"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	SegmentsDir     string `json:"segments_dir"`
	CurrentSegment  string `json:"current_segment"`
	SegmentMaxBytes int64  `json:"segment_max_bytes"`
	RecordsTotal    int    `json:"records_total"`
	LastHash        string `json:"last_hash"`
	PolicyRef       string `json:"policy_ref"`
	Notes           string `json:"notes"`
}

type ForwarderCursor struct {
	Segment string `json:"segment"`
	Line    int    `json:"line"`
}

type ForwarderDestination struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ForwarderState struct {
	Format      string               `json:"format"`
	UpdatedAt   string               `json:"updated_at"`
	Cursor      ForwarderCursor      `json:"cursor"`
	LastAckHash string               `json:"last_ack_hash"`
	Destination ForwarderDestination `json:"destination"`
	Notes       string               `json:"notes"`
}

type Metadata struct {
	Records   int
	LastHash  string
	PolicyRef string
}

type Record = EvidenceRecord

const (
	defaultEvidenceRoot          = "./data/evidence"
	defaultLegacyLogPath         = "./data/evidence.log"
	defaultSegmentMaxBytes int64 = 5_000_000
	segmentMaxBytesEnv           = "EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES"
	manifestFileName             = "manifest.json"
	segmentsDirName              = "segments"
	forwarderStateFileName       = "forwarder_state.json"
)

var appendMu sync.Mutex
var ErrChainInvalid = errors.New("evidence_chain_invalid")
var ErrCursorSegmentNotFound = errors.New("cursor_segment_not_found")
var ErrCursorLineOutOfRange = errors.New("cursor_line_out_of_range")
var errCursorResolved = errors.New("cursor_resolved")

type ChainValidationError struct {
	Index   int
	EventID string
	Message string
}

func (e *ChainValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.EventID != "" {
		return fmt.Sprintf("record %d (%s): %s", e.Index, e.EventID, e.Message)
	}
	return fmt.Sprintf("record %d: %s", e.Index, e.Message)
}

func ComputeHash(record EvidenceRecord) (string, error) {
	payload := canonicalEvidenceRecord{
		EventID:         record.EventID,
		Timestamp:       record.Timestamp.UTC(),
		PolicyRef:       record.PolicyRef,
		Actor:           record.Actor,
		Tool:            record.Tool,
		Operation:       record.Operation,
		Params:          record.Params,
		PolicyDecision:  record.PolicyDecision,
		ExecutionResult: record.ExecutionResult,
		PreviousHash:    record.PreviousHash,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal canonical record: %w", err)
	}

	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func Append(record EvidenceRecord) (EvidenceRecord, error) {
	return appendAtPath(defaultEvidenceRoot, record)
}

func ValidateChain() error {
	return validateChainAtPath(defaultEvidenceRoot)
}

func ValidateChainAtPath(path string) error {
	return validateChainAtPath(path)
}

func MetadataAtPath(path string) (Metadata, error) {
	mode, resolved, err := detectStoreMode(path)
	if err != nil {
		return Metadata{}, err
	}

	switch mode {
	case "segmented":
		m, err := loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), false)
		if err != nil {
			return Metadata{}, err
		}
		return Metadata{Records: m.RecordsTotal, LastHash: m.LastHash, PolicyRef: m.PolicyRef}, nil
	case "legacy":
		return metadataFromLegacyFile(resolved)
	default:
		return Metadata{}, fmt.Errorf("unsupported evidence store mode")
	}
}

func ReadAllAtPath(path string) ([]EvidenceRecord, error) {
	records := make([]EvidenceRecord, 0)
	err := ForEachRecordAtPath(path, func(rec Record) error {
		records = append(records, rec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func ForEachRecordAtPath(path string, fn func(Record) error) error {
	mode, resolved, err := detectStoreMode(path)
	if err != nil {
		return err
	}

	switch mode {
	case "segmented":
		return streamSegmentedRecords(resolved, fn)
	case "legacy":
		return streamLegacyRecords(resolved, fn)
	default:
		return fmt.Errorf("unsupported evidence store mode")
	}
}

func FindByEventID(path string, eventID string) (Record, bool, error) {
	if err := ValidateChainAtPath(path); err != nil {
		return Record{}, false, fmt.Errorf("%w: %v", ErrChainInvalid, err)
	}

	var foundRec Record
	found := false
	errFound := errors.New("record_found")
	err := ForEachRecordAtPath(path, func(rec Record) error {
		if rec.EventID == eventID {
			foundRec = rec
			found = true
			return errFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errFound) {
			return foundRec, true, nil
		}
		return Record{}, false, err
	}
	return Record{}, found, nil
}

func StoreFormatAtPath(path string) (string, error) {
	mode, _, err := detectStoreMode(path)
	if err != nil {
		return "", err
	}
	return mode, nil
}

func ManifestPath(root string) string {
	return filepath.Join(root, manifestFileName)
}

func ForwarderStatePath(root string) string {
	return filepath.Join(root, forwarderStateFileName)
}

func LoadManifest(path string) (StoreManifest, error) {
	mode, resolved, err := detectStoreMode(path)
	if err != nil {
		return StoreManifest{}, err
	}
	if mode != "segmented" {
		return StoreManifest{}, fmt.Errorf("manifest not available for legacy evidence store")
	}
	return loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), false)
}

func SegmentFiles(root string) ([]string, error) {
	mode, resolved, err := detectStoreMode(root)
	if err != nil {
		return nil, err
	}
	if mode != "segmented" {
		return nil, fmt.Errorf("segments not available for legacy evidence store")
	}
	_, names, err := orderedSegmentNames(resolved)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(names))
	for _, n := range names {
		files = append(files, filepath.Join(resolved, segmentsDirName, n))
	}
	return files, nil
}

func LoadForwarderState(root string) (ForwarderState, bool, error) {
	mode, resolved, err := detectStoreMode(root)
	if err != nil {
		return ForwarderState{}, false, err
	}
	if mode != "segmented" {
		return ForwarderState{}, false, fmt.Errorf("cursor not supported for legacy evidence")
	}

	raw, err := os.ReadFile(ForwarderStatePath(resolved))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ForwarderState{}, false, nil
		}
		return ForwarderState{}, false, err
	}

	var state ForwarderState
	if err := json.Unmarshal(raw, &state); err != nil {
		return ForwarderState{}, false, fmt.Errorf("parse forwarder state: %w", err)
	}
	return state, true, nil
}

func SaveForwarderState(root string, state ForwarderState) error {
	mode, resolved, err := detectStoreMode(root)
	if err != nil {
		return err
	}
	if mode != "segmented" {
		return fmt.Errorf("cursor not supported for legacy evidence")
	}

	if state.Format == "" {
		state.Format = "evidra-forwarder-state-v0.1"
	}
	if state.Destination.Type == "" {
		state.Destination.Type = "none"
	}

	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return fmt.Errorf("create evidence root: %w", err)
	}

	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal forwarder state: %w", err)
	}

	path := ForwarderStatePath(resolved)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write forwarder state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename forwarder state tmp: %w", err)
	}
	return nil
}

func ResolveCursorRecord(root, segment string, line int) (Record, error) {
	if line < 0 {
		return Record{}, ErrCursorLineOutOfRange
	}

	mode, resolved, err := detectStoreMode(root)
	if err != nil {
		return Record{}, err
	}
	if mode != "segmented" {
		return Record{}, fmt.Errorf("cursor not supported for legacy evidence")
	}

	segPath := filepath.Join(resolved, segmentsDirName, segment)
	if _, err := os.Stat(segPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Record{}, ErrCursorSegmentNotFound
		}
		return Record{}, err
	}

	current := 0
	var out Record
	found := false
	err = streamFileRecords(segPath, func(rec Record, _ int) error {
		if current == line {
			out = rec
			found = true
			return errCursorResolved
		}
		current++
		return nil
	})
	if err != nil {
		if errors.Is(err, errCursorResolved) {
			return out, nil
		}
		return Record{}, err
	}
	if !found {
		return Record{}, ErrCursorLineOutOfRange
	}
	return out, nil
}

func appendAtPath(path string, record EvidenceRecord) (EvidenceRecord, error) {
	appendMu.Lock()
	defer appendMu.Unlock()

	mode, resolved, err := detectStoreMode(path)
	if err != nil {
		return EvidenceRecord{}, err
	}

	switch mode {
	case "segmented":
		return appendSegmented(resolved, record)
	case "legacy":
		return appendLegacy(resolved, record)
	default:
		return EvidenceRecord{}, fmt.Errorf("unsupported evidence store mode")
	}
}

func validateChainAtPath(path string) error {
	appendMu.Lock()
	defer appendMu.Unlock()

	mode, resolved, err := detectStoreMode(path)
	if err != nil {
		return err
	}

	switch mode {
	case "segmented":
		return validateSegmentedChain(resolved)
	case "legacy":
		return validateLegacyChain(resolved)
	default:
		return fmt.Errorf("unsupported evidence store mode")
	}
}

func detectStoreMode(path string) (string, string, error) {
	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err == nil {
		if info.IsDir() {
			return "segmented", clean, nil
		}
		return "legacy", clean, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}

	ext := strings.ToLower(filepath.Ext(clean))
	if ext == ".log" || ext == ".jsonl" {
		return "legacy", clean, nil
	}
	return "segmented", clean, nil
}

func segmentMaxBytesFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv(segmentMaxBytesEnv))
	if raw == "" {
		return defaultSegmentMaxBytes
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return defaultSegmentMaxBytes
	}
	return v
}

func appendSegmented(root string, record EvidenceRecord) (EvidenceRecord, error) {
	maxBytes := segmentMaxBytesFromEnv()
	manifest, err := loadOrInitManifest(root, maxBytes, true)
	if err != nil {
		return EvidenceRecord{}, err
	}
	if manifest.SegmentMaxBytes <= 0 {
		manifest.SegmentMaxBytes = maxBytes
	}
	if manifest.CurrentSegment == "" {
		manifest.CurrentSegment = segmentName(1)
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}
	record.PreviousHash = manifest.LastHash

	hash, err := ComputeHash(record)
	if err != nil {
		return EvidenceRecord{}, err
	}
	record.Hash = hash

	segPath := filepath.Join(root, segmentsDirName, manifest.CurrentSegment)
	if err := os.MkdirAll(filepath.Dir(segPath), 0o755); err != nil {
		return EvidenceRecord{}, fmt.Errorf("create segments directory: %w", err)
	}
	if err := appendRecordLine(segPath, record); err != nil {
		return EvidenceRecord{}, err
	}

	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.RecordsTotal++
	manifest.LastHash = record.Hash
	if manifest.PolicyRef == "" && record.PolicyRef != "" {
		manifest.PolicyRef = record.PolicyRef
	} else if manifest.PolicyRef != "" && record.PolicyRef != "" && manifest.PolicyRef != record.PolicyRef {
		manifest.PolicyRef = ""
	}

	info, err := os.Stat(segPath)
	if err == nil && info.Size() > manifest.SegmentMaxBytes {
		_, names, listErr := orderedSegmentNames(root)
		if listErr != nil {
			return EvidenceRecord{}, listErr
		}
		next := 1
		if len(names) > 0 {
			lastIndex, parseErr := parseSegmentIndex(names[len(names)-1])
			if parseErr != nil {
				return EvidenceRecord{}, parseErr
			}
			next = lastIndex + 1
		}
		manifest.CurrentSegment = segmentName(next)
		nextPath := filepath.Join(root, segmentsDirName, manifest.CurrentSegment)
		if _, err := os.Stat(nextPath); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(nextPath, []byte(""), 0o644); err != nil {
				return EvidenceRecord{}, fmt.Errorf("create next segment: %w", err)
			}
		}
	}

	if err := writeManifestAtomic(root, manifest); err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func appendLegacy(path string, record EvidenceRecord) (EvidenceRecord, error) {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	last, ok, err := readLastRecord(path)
	if err != nil {
		return EvidenceRecord{}, err
	}
	if ok {
		record.PreviousHash = last.Hash
	} else {
		record.PreviousHash = ""
	}

	hash, err := ComputeHash(record)
	if err != nil {
		return EvidenceRecord{}, err
	}
	record.Hash = hash

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return EvidenceRecord{}, fmt.Errorf("create log directory: %w", err)
	}
	if err := appendRecordLine(path, record); err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func validateSegmentedChain(root string) error {
	manifest, err := loadOrInitManifest(root, segmentMaxBytesFromEnv(), false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(root); statErr == nil {
				return fmt.Errorf("manifest not found")
			}
			return nil
		}
		return err
	}

	_, names, err := orderedSegmentNames(root)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		if manifest.RecordsTotal != 0 || manifest.LastHash != "" {
			return fmt.Errorf("manifest indicates records but no segments exist")
		}
		return nil
	}

	var prev string
	total := 0
	lastHash := ""
	policyRef := ""

	for _, name := range names {
		segPath := filepath.Join(root, segmentsDirName, name)
		err := streamFileRecords(segPath, func(rec Record, _ int) error {
			if total == 0 {
				if rec.PreviousHash != "" {
					return &ChainValidationError{Index: total, EventID: rec.EventID, Message: "non-empty previous_hash in chain head"}
				}
			} else if rec.PreviousHash != prev {
				return &ChainValidationError{Index: total, EventID: rec.EventID, Message: "previous_hash mismatch"}
			}

			expected, err := ComputeHash(rec)
			if err != nil {
				return fmt.Errorf("compute hash for record %d: %w", total, err)
			}
			if rec.Hash != expected {
				return &ChainValidationError{Index: total, EventID: rec.EventID, Message: "hash mismatch"}
			}

			if rec.PolicyRef != "" {
				if policyRef == "" {
					policyRef = rec.PolicyRef
				} else if policyRef != rec.PolicyRef {
					return fmt.Errorf("mixed policy_ref values detected")
				}
			}

			prev = rec.Hash
			lastHash = rec.Hash
			total++
			return nil
		})
		if err != nil {
			return err
		}
	}

	if total != manifest.RecordsTotal {
		return fmt.Errorf("manifest records_total mismatch")
	}
	if lastHash != manifest.LastHash {
		return fmt.Errorf("manifest last_hash mismatch")
	}
	if manifest.CurrentSegment != names[len(names)-1] {
		return fmt.Errorf("manifest current_segment mismatch")
	}
	if manifest.PolicyRef != "" && policyRef != "" && manifest.PolicyRef != policyRef {
		return fmt.Errorf("manifest policy_ref mismatch")
	}

	return nil
}

func validateLegacyChain(path string) error {
	var prev string
	idx := 0
	policyRef := ""
	err := streamFileRecords(path, func(rec Record, _ int) error {
		if idx == 0 {
			if rec.PreviousHash != "" {
				return &ChainValidationError{Index: idx, EventID: rec.EventID, Message: "non-empty previous_hash in chain head"}
			}
		} else if rec.PreviousHash != prev {
			return &ChainValidationError{Index: idx, EventID: rec.EventID, Message: "previous_hash mismatch"}
		}

		expected, err := ComputeHash(rec)
		if err != nil {
			return fmt.Errorf("compute hash for record %d: %w", idx, err)
		}
		if rec.Hash != expected {
			return &ChainValidationError{Index: idx, EventID: rec.EventID, Message: "hash mismatch"}
		}
		if rec.PolicyRef != "" {
			if policyRef == "" {
				policyRef = rec.PolicyRef
			} else if policyRef != rec.PolicyRef {
				return fmt.Errorf("mixed policy_ref values detected")
			}
		}
		prev = rec.Hash
		idx++
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func metadataFromLegacyFile(path string) (Metadata, error) {
	meta := Metadata{}
	policyRef := ""
	err := streamFileRecords(path, func(rec Record, _ int) error {
		meta.Records++
		meta.LastHash = rec.Hash
		if rec.PolicyRef != "" {
			if policyRef == "" {
				policyRef = rec.PolicyRef
			} else if policyRef != rec.PolicyRef {
				return fmt.Errorf("mixed policy_ref values detected")
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{}, nil
		}
		return Metadata{}, err
	}
	meta.PolicyRef = policyRef
	return meta, nil
}

func streamSegmentedRecords(root string, fn func(Record) error) error {
	_, names, err := orderedSegmentNames(root)
	if err != nil {
		return err
	}
	for _, name := range names {
		segPath := filepath.Join(root, segmentsDirName, name)
		err := streamFileRecords(segPath, func(rec Record, _ int) error {
			return fn(rec)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func streamLegacyRecords(path string, fn func(Record) error) error {
	return streamFileRecords(path, func(rec Record, _ int) error {
		return fn(rec)
	})
}

func streamFileRecords(path string, fn func(Record, int) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return fmt.Errorf("parse JSONL line %d: %w", lineNo, err)
		}
		if err := fn(rec, lineNo); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read JSONL: %w", err)
	}
	return nil
}

func appendRecordLine(path string, record Record) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open evidence log: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append record: %w", err)
	}
	return nil
}

func orderedSegmentNames(root string) ([]int, []string, error) {
	segDir := filepath.Join(root, segmentsDirName)
	matches, err := filepath.Glob(filepath.Join(segDir, "evidence-*.jsonl"))
	if err != nil {
		return nil, nil, err
	}
	if len(matches) == 0 {
		return nil, nil, nil
	}

	names := make([]string, 0, len(matches))
	indices := make([]int, 0, len(matches))
	for _, m := range matches {
		name := filepath.Base(m)
		idx, err := parseSegmentIndex(name)
		if err != nil {
			return nil, nil, err
		}
		names = append(names, name)
		indices = append(indices, idx)
	}

	sort.SliceStable(names, func(i, j int) bool { return names[i] < names[j] })
	sort.Ints(indices)

	for i, idx := range indices {
		expected := i + 1
		if idx != expected {
			return nil, nil, fmt.Errorf("missing segment in sequence: expected %s", segmentName(expected))
		}
	}

	for i, name := range names {
		expected := segmentName(i + 1)
		if name != expected {
			return nil, nil, fmt.Errorf("unexpected segment name: %s", name)
		}
	}

	return indices, names, nil
}

func parseSegmentIndex(name string) (int, error) {
	var idx int
	n, err := fmt.Sscanf(name, "evidence-%06d.jsonl", &idx)
	if err != nil || n != 1 || idx <= 0 {
		return 0, fmt.Errorf("invalid segment filename: %s", name)
	}
	if name != segmentName(idx) {
		return 0, fmt.Errorf("invalid segment filename: %s", name)
	}
	return idx, nil
}

func segmentName(idx int) string {
	return fmt.Sprintf("evidence-%06d.jsonl", idx)
}

func loadOrInitManifest(root string, segmentMaxBytes int64, createIfMissing bool) (StoreManifest, error) {
	manifestPath := ManifestPath(root)
	raw, err := os.ReadFile(manifestPath)
	if err == nil {
		var m StoreManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return StoreManifest{}, fmt.Errorf("parse manifest: %w", err)
		}
		if m.SegmentMaxBytes <= 0 {
			m.SegmentMaxBytes = segmentMaxBytes
		}
		if m.SegmentsDir == "" {
			m.SegmentsDir = segmentsDirName
		}
		if m.CurrentSegment == "" {
			m.CurrentSegment = segmentName(1)
		}
		return m, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return StoreManifest{}, err
	}
	if !createIfMissing {
		return StoreManifest{}, os.ErrNotExist
	}

	now := time.Now().UTC().Format(time.RFC3339)
	m := StoreManifest{
		Format:          "evidra-evidence-manifest-v0.1",
		CreatedAt:       now,
		UpdatedAt:       now,
		SegmentsDir:     segmentsDirName,
		CurrentSegment:  segmentName(1),
		SegmentMaxBytes: segmentMaxBytes,
		RecordsTotal:    0,
		LastHash:        "",
		PolicyRef:       "",
		Notes:           "Local segmented evidence store",
	}
	if createIfMissing {
		if err := os.MkdirAll(filepath.Join(root, segmentsDirName), 0o755); err != nil {
			return StoreManifest{}, fmt.Errorf("create segments directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(root, segmentsDirName, m.CurrentSegment), []byte(""), 0o644); err != nil {
			return StoreManifest{}, fmt.Errorf("create first segment: %w", err)
		}
		if err := writeManifestAtomic(root, m); err != nil {
			return StoreManifest{}, err
		}
	}
	return m, nil
}

func writeManifestAtomic(root string, manifest StoreManifest) error {
	if manifest.Format == "" {
		manifest.Format = "evidra-evidence-manifest-v0.1"
	}
	if manifest.SegmentsDir == "" {
		manifest.SegmentsDir = segmentsDirName
	}
	if manifest.CurrentSegment == "" {
		manifest.CurrentSegment = segmentName(1)
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create evidence root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, segmentsDirName), 0o755); err != nil {
		return fmt.Errorf("create segments directory: %w", err)
	}

	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := ManifestPath(root)
	tmpPath := manifestPath + ".tmp"
	if err := os.WriteFile(tmpPath, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest tmp: %w", err)
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return fmt.Errorf("rename manifest tmp: %w", err)
	}
	return nil
}

func readLastRecord(path string) (EvidenceRecord, bool, error) {
	records, err := readAllRecords(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EvidenceRecord{}, false, nil
		}
		return EvidenceRecord{}, false, err
	}
	if len(records) == 0 {
		return EvidenceRecord{}, false, nil
	}
	return records[len(records)-1], true, nil
}

func readAllRecords(path string) ([]EvidenceRecord, error) {
	records := make([]EvidenceRecord, 0)
	err := ForEachRecordAtPath(path, func(rec Record) error {
		records = append(records, rec)
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	return records, nil
}
