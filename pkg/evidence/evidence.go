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
)

type EvidenceRecord struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor"`
	Action    string                 `json:"action"`
	Subject   string                 `json:"subject"`
	Details   map[string]interface{} `json:"details,omitempty"`
	PrevHash  string                 `json:"prev_hash,omitempty"`
	Hash      string                 `json:"hash"`
}

type canonicalEvidenceRecord struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor"`
	Action    string                 `json:"action"`
	Subject   string                 `json:"subject"`
	Details   map[string]interface{} `json:"details,omitempty"`
	PrevHash  string                 `json:"prev_hash,omitempty"`
}

const logPath = "./data/evidence.log"

var appendMu sync.Mutex

func ComputeHash(record EvidenceRecord) (string, error) {
	payload := canonicalEvidenceRecord{
		ID:        record.ID,
		Timestamp: record.Timestamp.UTC(),
		Actor:     record.Actor,
		Action:    record.Action,
		Subject:   record.Subject,
		Details:   record.Details,
		PrevHash:  record.PrevHash,
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
		record.PrevHash = last.Hash
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
			if rec.PrevHash != "" {
				return fmt.Errorf("record %d has non-empty prev_hash in chain head", i)
			}
		} else if rec.PrevHash != prev {
			return fmt.Errorf("record %d prev_hash mismatch", i)
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
