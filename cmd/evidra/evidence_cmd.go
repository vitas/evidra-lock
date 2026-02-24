package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"samebits.com/evidra/pkg/evidence"
	"samebits.com/evidra/pkg/version"
)

const (
	exitOK            = 0
	exitVerifyFailed  = 1
	exitInputError    = 2
	exitExportFailure = 3
)

type verifyOKOutput struct {
	OK       bool   `json:"ok"`
	Records  int    `json:"records"`
	LastHash string `json:"last_hash"`
}

type verifyFailOutput struct {
	OK       bool        `json:"ok"`
	Code     string      `json:"code,omitempty"`
	Error    string      `json:"error"`
	FailedAt interface{} `json:"failed_at,omitempty"`
}

type manifest struct {
	Format                        string `json:"format"`
	CreatedAt                     string `json:"created_at"`
	EvidenceFile                  string `json:"evidence_file"`
	Records                       int    `json:"records"`
	LastHash                      string `json:"last_hash"`
	PolicyRef                     string `json:"policy_ref"`
	Notes                         string `json:"notes"`
	EvidenceStoreFormat           string `json:"evidence_store_format"`
	EvidenceStoreManifestLastHash string `json:"evidence_store_manifest_last_hash,omitempty"`

	PolicyFileSHA256 string `json:"policy_file_sha256,omitempty"`
	DataFileSHA256   string `json:"data_file_sha256,omitempty"`
}

func runEvidenceCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && strings.TrimSpace(args[0]) == "--version" {
		fmt.Fprintf(stdout, "evidra evidence %s\n", version.Version)
		return exitOK
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evidra evidence <verify|export|violations|cursor> [flags]")
		return exitInputError
	}

	switch args[0] {
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "violations":
		return runViolations(args[1:], stdout, stderr)
	case "cursor":
		return runCursor(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: evidra evidence <verify|export|violations|cursor> [flags]")
		return exitInputError
	}
}

func runVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidencePath := fs.String("evidence", "", "Path to evidence log")
	if err := fs.Parse(args); err != nil {
		return exitInputError
	}
	if *evidencePath == "" {
		fmt.Fprintln(stderr, "--evidence is required")
		return exitInputError
	}
	if _, err := os.Stat(*evidencePath); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}

	if err := evidence.ValidateChainAtPath(*evidencePath); err != nil {
		out := verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		}
		if code := evidence.ErrorCode(err); code != "" {
			out.Code = code
		}
		var chainErr *evidence.ChainValidationError
		if errors.As(err, &chainErr) {
			if chainErr.EventID != "" {
				out.FailedAt = chainErr.EventID
			} else {
				out.FailedAt = chainErr.Index
			}
		}
		_ = writeJSON(stdout, out)
		return exitVerifyFailed
	}

	meta, err := evidence.MetadataAtPath(*evidencePath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	_ = writeJSON(stdout, verifyOKOutput{
		OK:       true,
		Records:  meta.Records,
		LastHash: meta.LastHash,
	})
	return exitOK
}

func runExport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidencePath := fs.String("evidence", "", "Path to evidence log")
	outPath := fs.String("out", "", "Output tar.gz path")
	policyPath := fs.String("policy", "", "Optional policy.rego path")
	dataPath := fs.String("data", "", "Optional policy data JSON path")
	if err := fs.Parse(args); err != nil {
		return exitInputError
	}
	if *evidencePath == "" || *outPath == "" {
		fmt.Fprintln(stderr, "--evidence and --out are required")
		return exitInputError
	}
	if _, err := os.Stat(*evidencePath); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	if *policyPath != "" {
		if _, err := os.Stat(*policyPath); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
	}
	if *dataPath != "" {
		if _, err := os.Stat(*dataPath); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
	}

	if err := evidence.ValidateChainAtPath(*evidencePath); err != nil {
		out := verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		}
		if code := evidence.ErrorCode(err); code != "" {
			out.Code = code
		}
		_ = writeJSON(stdout, out)
		return exitVerifyFailed
	}

	meta, err := evidence.MetadataAtPath(*evidencePath)
	if err != nil {
		out := verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		}
		if code := evidence.ErrorCode(err); code != "" {
			out.Code = code
		}
		_ = writeJSON(stdout, out)
		return exitVerifyFailed
	}

	storeFormat, err := evidence.StoreFormatAtPath(*evidencePath)
	if err != nil {
		out := verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		}
		if code := evidence.ErrorCode(err); code != "" {
			out.Code = code
		}
		_ = writeJSON(stdout, out)
		return exitExportFailure
	}

	var policyBytes []byte
	if *policyPath != "" {
		policyBytes, err = os.ReadFile(*policyPath)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
	}

	var dataBytes []byte
	if *dataPath != "" {
		dataBytes, err = os.ReadFile(*dataPath)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
	}

	evidenceFileRef := "evidence/evidence.log"
	storeManifestLastHash := ""
	if storeFormat == "segmented" {
		evidenceFileRef = "evidence/manifest.json"
		storeManifest, err := evidence.LoadManifest(*evidencePath)
		if err != nil {
			out := verifyFailOutput{
				OK:    false,
				Error: err.Error(),
			}
			if code := evidence.ErrorCode(err); code != "" {
				out.Code = code
			}
			_ = writeJSON(stdout, out)
			return exitExportFailure
		}
		storeManifestLastHash = storeManifest.LastHash
	}

	m := manifest{
		Format:                        "evidra-audit-pack-v0.1",
		CreatedAt:                     time.Now().UTC().Format(time.RFC3339),
		EvidenceFile:                  evidenceFileRef,
		Records:                       meta.Records,
		LastHash:                      meta.LastHash,
		PolicyRef:                     meta.PolicyRef,
		Notes:                         "Evidra audit pack v0.1",
		EvidenceStoreFormat:           storeFormat,
		EvidenceStoreManifestLastHash: storeManifestLastHash,
	}
	if len(policyBytes) > 0 {
		m.PolicyFileSHA256 = sha256Hex(policyBytes)
	}
	if len(dataBytes) > 0 {
		m.DataFileSHA256 = sha256Hex(dataBytes)
	}

	if err := writeAuditPack(*outPath, *evidencePath, storeFormat, policyBytes, dataBytes, m); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitExportFailure
	}
	_ = writeJSON(stdout, map[string]interface{}{
		"ok":  true,
		"out": *outPath,
	})
	return exitOK
}

type violationsTimeWindow struct {
	Since *string `json:"since"`
	From  *string `json:"from"`
	To    *string `json:"to"`
}

type violationsFilters struct {
	MinRisk       string `json:"min_risk"`
	IncludeDenies bool   `json:"include_denies"`
}

type reasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type toolCount struct {
	Tool      string `json:"tool"`
	Operation string `json:"operation"`
	Count     int    `json:"count"`
}

type actorCount struct {
	ActorID string `json:"actor_id"`
	Count   int    `json:"count"`
}

type sampleEvent struct {
	EventID   string `json:"event_id"`
	Timestamp string `json:"timestamp"`
	Tool      string `json:"tool"`
	Operation string `json:"operation"`
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
}

type violationsReport struct {
	OK              bool                 `json:"ok"`
	EvidencePath    string               `json:"evidence_path"`
	RecordsTotal    int                  `json:"records_total"`
	RecordsScanned  int                  `json:"records_scanned"`
	TimeWindow      violationsTimeWindow `json:"time_window"`
	Filters         violationsFilters    `json:"filters"`
	ViolationsTotal int                  `json:"violations_total"`
	ByReason        []reasonCount        `json:"by_reason"`
	ByTool          []toolCount          `json:"by_tool"`
	TopActors       []actorCount         `json:"top_actors"`
	Samples         []sampleEvent        `json:"samples"`
}

func runViolations(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("violations", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidencePath := fs.String("evidence", "", "Path to evidence log")
	since := fs.String("since", "", "Optional duration window (e.g. 24h)")
	minRisk := fs.String("min-risk", "high", "Minimum risk level: low|medium|high")
	_ = fs.Bool("json", true, "Output JSON")
	if err := fs.Parse(args); err != nil {
		return exitInputError
	}
	if *evidencePath == "" {
		fmt.Fprintln(stderr, "--evidence is required")
		return exitInputError
	}
	if _, err := os.Stat(*evidencePath); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}

	minRiskRank, ok := riskRank(*minRisk)
	if !ok {
		fmt.Fprintln(stderr, "--min-risk must be one of: low|medium|high")
		return exitInputError
	}

	var sinceDuration time.Duration
	var err error
	var sincePtr *string
	var fromPtr *string
	if strings.TrimSpace(*since) != "" {
		sinceDuration, err = time.ParseDuration(strings.TrimSpace(*since))
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
		s := strings.TrimSpace(*since)
		sincePtr = &s
		from := time.Now().UTC().Add(-sinceDuration).Format(time.RFC3339)
		fromPtr = &from
	}

	if err := evidence.ValidateChainAtPath(*evidencePath); err != nil {
		var chainErr *evidence.ChainValidationError
		out := map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		}
		if errors.As(err, &chainErr) {
			out["hint"] = "evidence chain is invalid"
			_ = writeJSON(stdout, out)
			return exitVerifyFailed
		}
		out["hint"] = "evidence parse or internal error"
		_ = writeJSON(stdout, out)
		return exitExportFailure
	}

	meta, err := evidence.MetadataAtPath(*evidencePath)
	if err != nil {
		_ = writeJSON(stdout, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
			"hint":  "failed to read evidence metadata",
		})
		return exitExportFailure
	}

	now := time.Now().UTC()
	var cutoff time.Time
	if sincePtr != nil {
		cutoff = now.Add(-sinceDuration)
	}

	reasonCounts := make(map[string]int)
	toolCounts := make(map[string]int)
	actorCounts := make(map[string]int)
	samples := make([]sampleEvent, 0, 10)
	recordsScanned := 0
	violationsTotal := 0

	err = evidence.ForEachRecordAtPath(*evidencePath, func(rec evidence.Record) error {
		ts := rec.Timestamp.UTC()
		if sincePtr != nil && ts.Before(cutoff) {
			return nil
		}
		recordsScanned++

		recRiskRank, _ := riskRank(rec.PolicyDecision.RiskLevel)
		isCandidate := !rec.PolicyDecision.Allow || recRiskRank >= minRiskRank
		if !isCandidate {
			return nil
		}
		violationsTotal++

		reason := rec.PolicyDecision.Reason
		if reason == "" {
			reason = "unknown_reason"
		}
		reasonCounts[reason]++

		toolKey := rec.Tool + "|" + rec.Operation
		toolCounts[toolKey]++

		actorID := rec.Actor.ID
		if actorID == "" {
			actorID = "unknown"
		}
		actorCounts[actorID]++

		if len(samples) < 10 {
			samples = append(samples, sampleEvent{
				EventID:   rec.EventID,
				Timestamp: ts.Format(time.RFC3339),
				Tool:      rec.Tool,
				Operation: rec.Operation,
				Allow:     rec.PolicyDecision.Allow,
				RiskLevel: normalizedRiskLevel(rec.PolicyDecision.RiskLevel),
				Reason:    reason,
			})
		}
		return nil
	})
	if err != nil {
		_ = writeJSON(stdout, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
			"hint":  "failed to parse evidence records",
		})
		return exitExportFailure
	}

	byReason := buildReasonCounts(reasonCounts, 50)
	byTool := buildToolCounts(toolCounts, 50)
	topActors := buildActorCounts(actorCounts, 10)

	to := now.Format(time.RFC3339)
	report := violationsReport{
		OK:             true,
		EvidencePath:   *evidencePath,
		RecordsTotal:   meta.Records,
		RecordsScanned: recordsScanned,
		TimeWindow: violationsTimeWindow{
			Since: sincePtr,
			From:  fromPtr,
			To:    &to,
		},
		Filters: violationsFilters{
			MinRisk:       normalizedRiskLevel(*minRisk),
			IncludeDenies: true,
		},
		ViolationsTotal: violationsTotal,
		ByReason:        byReason,
		ByTool:          byTool,
		TopActors:       topActors,
		Samples:         samples,
	}
	_ = writeJSON(stdout, report)
	return exitOK
}

func runCursor(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evidra evidence cursor <show|ack> [flags]")
		return exitInputError
	}
	switch args[0] {
	case "show":
		return runCursorShow(args[1:], stdout, stderr)
	case "ack":
		return runCursorAck(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: evidra evidence cursor <show|ack> [flags]")
		return exitInputError
	}
}

func runCursorShow(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("cursor show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidencePath := fs.String("evidence", "", "Path to segmented evidence root")
	if err := fs.Parse(args); err != nil {
		return exitInputError
	}
	if *evidencePath == "" {
		fmt.Fprintln(stderr, "--evidence is required")
		return exitInputError
	}
	info, err := os.Stat(*evidencePath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	if !info.IsDir() {
		fmt.Fprintln(stderr, "cursor not supported for legacy evidence")
		return exitInputError
	}

	storeFormat, err := evidence.StoreFormatAtPath(*evidencePath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	if storeFormat != "segmented" {
		fmt.Fprintln(stderr, "cursor not supported for legacy evidence")
		return exitInputError
	}

	state, found, err := evidence.LoadForwarderState(*evidencePath)
	if err != nil {
		_ = writeJSON(stdout, map[string]interface{}{"ok": false, "error": err.Error()})
		return exitExportFailure
	}
	if !found {
		_ = writeJSON(stdout, map[string]interface{}{"ok": true, "cursor": nil})
		return exitOK
	}

	_ = writeJSON(stdout, map[string]interface{}{
		"ok":            true,
		"cursor":        state.Cursor,
		"last_ack_hash": state.LastAckHash,
	})
	return exitOK
}

func runCursorAck(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("cursor ack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidencePath := fs.String("evidence", "", "Path to segmented evidence root")
	segment := fs.String("segment", "", "Segment filename")
	line := fs.Int("line", -1, "Line index (0-based)")
	if err := fs.Parse(args); err != nil {
		return exitInputError
	}
	if *evidencePath == "" || *segment == "" || *line < 0 {
		fmt.Fprintln(stderr, "--evidence, --segment, and --line are required")
		return exitInputError
	}
	info, err := os.Stat(*evidencePath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	if !info.IsDir() {
		fmt.Fprintln(stderr, "cursor not supported for legacy evidence")
		return exitInputError
	}

	storeFormat, err := evidence.StoreFormatAtPath(*evidencePath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitInputError
	}
	if storeFormat != "segmented" {
		fmt.Fprintln(stderr, "cursor not supported for legacy evidence")
		return exitInputError
	}

	if err := evidence.ValidateChainAtPath(*evidencePath); err != nil {
		_ = writeJSON(stdout, map[string]interface{}{"ok": false, "error": err.Error()})
		return exitVerifyFailed
	}

	rec, err := evidence.ResolveCursorRecord(*evidencePath, *segment, *line)
	if err != nil {
		if errors.Is(err, evidence.ErrCursorSegmentNotFound) || errors.Is(err, evidence.ErrCursorLineOutOfRange) {
			fmt.Fprintln(stderr, err.Error())
			return exitInputError
		}
		_ = writeJSON(stdout, map[string]interface{}{"ok": false, "error": err.Error()})
		return exitExportFailure
	}

	state := evidence.ForwarderState{
		Format:      "evidra-forwarder-state-v0.1",
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Cursor:      evidence.ForwarderCursor{Segment: *segment, Line: *line},
		LastAckHash: rec.Hash,
		Destination: evidence.ForwarderDestination{Type: "none", ID: ""},
		Notes:       "",
	}
	if err := evidence.SaveForwarderState(*evidencePath, state); err != nil {
		_ = writeJSON(stdout, map[string]interface{}{"ok": false, "error": err.Error()})
		return exitExportFailure
	}

	_ = writeJSON(stdout, map[string]interface{}{
		"ok":            true,
		"cursor":        state.Cursor,
		"last_ack_hash": state.LastAckHash,
	})
	return exitOK
}

func writeAuditPack(outPath, evidencePath, storeFormat string, policyBytes []byte, dataBytes []byte, m manifest) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	switch storeFormat {
	case "legacy":
		if err := addFileToTar(tw, evidencePath, "evidence/evidence.log"); err != nil {
			return err
		}
	case "segmented":
		manifestPath := evidence.ManifestPath(evidencePath)
		if err := addFileToTar(tw, manifestPath, "evidence/manifest.json"); err != nil {
			return err
		}
		segments, err := evidence.SegmentFiles(evidencePath)
		if err != nil {
			return err
		}
		for _, seg := range segments {
			dest := filepath.ToSlash(filepath.Join("evidence", "segments", filepath.Base(seg)))
			if err := addFileToTar(tw, seg, dest); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported evidence store format: %s", storeFormat)
	}

	manifestBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := addBytesToTar(tw, "manifest.json", manifestBytes); err != nil {
		return err
	}

	if len(policyBytes) > 0 {
		if err := addBytesToTar(tw, "policy/active.rego", policyBytes); err != nil {
			return err
		}
	}
	if len(dataBytes) > 0 {
		if err := addBytesToTar(tw, "policy/active-data.json", dataBytes); err != nil {
			return err
		}
	}

	return nil
}

func addFileToTar(tw *tar.Writer, srcPath, destPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}

	hdr := &tar.Header{
		Name:    filepath.ToSlash(destPath),
		Mode:    0o644,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", destPath, err)
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("copy %s into tar: %w", srcPath, err)
	}
	return nil
}

func addBytesToTar(tw *tar.Writer, destPath string, content []byte) error {
	hdr := &tar.Header{
		Name:    filepath.ToSlash(destPath),
		Mode:    0o644,
		Size:    int64(len(content)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", destPath, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write tar file %s: %w", destPath, err)
	}
	return nil
}

func writeJSON(w io.Writer, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func riskRank(level string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low":
		return 1, true
	case "medium":
		return 2, true
	case "high":
		return 3, true
	default:
		return 3, false
	}
}

func normalizedRiskLevel(level string) string {
	lvl := strings.ToLower(strings.TrimSpace(level))
	switch lvl {
	case "low", "medium", "high":
		return lvl
	default:
		return "high"
	}
}

func buildReasonCounts(input map[string]int, limit int) []reasonCount {
	out := make([]reasonCount, 0, len(input))
	for reason, count := range input {
		out = append(out, reasonCount{Reason: reason, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Reason < out[j].Reason
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildToolCounts(input map[string]int, limit int) []toolCount {
	out := make([]toolCount, 0, len(input))
	for key, count := range input {
		parts := strings.SplitN(key, "|", 2)
		tool := parts[0]
		op := ""
		if len(parts) == 2 {
			op = parts[1]
		}
		out = append(out, toolCount{Tool: tool, Operation: op, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			if out[i].Tool == out[j].Tool {
				return out[i].Operation < out[j].Operation
			}
			return out[i].Tool < out[j].Tool
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildActorCounts(input map[string]int, limit int) []actorCount {
	out := make([]actorCount, 0, len(input))
	for actorID, count := range input {
		out = append(out, actorCount{ActorID: actorID, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].ActorID < out[j].ActorID
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
