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
	"strings"
	"sync"
	"time"

	"samebits.com/evidra-mcp/pkg/invocation"
)

type PolicyDecision struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
}

type ExecutionResult struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

type EvidenceRecord struct {
	EventID         string                 `json:"event_id"`
	Timestamp       time.Time              `json:"timestamp"`
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
	Actor           invocation.Actor       `json:"actor"`
	Tool            string                 `json:"tool"`
	Operation       string                 `json:"operation"`
	Params          map[string]interface{} `json:"params"`
	PolicyDecision  PolicyDecision         `json:"policy_decision"`
	ExecutionResult ExecutionResult        `json:"execution_result"`
	PreviousHash    string                 `json:"previous_hash"`
}

const logPath = "./data/evidence.log"

var appendMu sync.Mutex

func ComputeHash(record EvidenceRecord) (string, error) {
	payload := canonicalEvidenceRecord{
		EventID:         record.EventID,
		Timestamp:       record.Timestamp.UTC(),
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
	appendMu.Lock()
	defer appendMu.Unlock()

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	last, ok, err := readLastRecord(logPath)
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

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return EvidenceRecord{}, fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return EvidenceRecord{}, fmt.Errorf("open evidence log: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(record)
	if err != nil {
		return EvidenceRecord{}, fmt.Errorf("marshal record: %w", err)
	}

	if _, err := f.Write(append(line, '\n')); err != nil {
		return EvidenceRecord{}, fmt.Errorf("append record: %w", err)
	}

	return record, nil
}

func ValidateChain() error {
	appendMu.Lock()
	defer appendMu.Unlock()

	records, err := readAllRecords(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var prev string
	for i, rec := range records {
		if i == 0 {
			if rec.PreviousHash != "" {
				return fmt.Errorf("record %d has non-empty previous_hash in chain head", i)
			}
		} else if rec.PreviousHash != prev {
			return fmt.Errorf("record %d previous_hash mismatch", i)
		}

		expected, err := ComputeHash(rec)
		if err != nil {
			return fmt.Errorf("compute hash for record %d: %w", i, err)
		}
		if rec.Hash != expected {
			return fmt.Errorf("record %d hash mismatch", i)
		}
		prev = rec.Hash
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records := make([]EvidenceRecord, 0)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec EvidenceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("parse JSONL line %d: %w", lineNo, err)
		}
		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read JSONL: %w", err)
	}

	return records, nil
}
