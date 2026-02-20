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
	"time"

	"samebits.com/evidra-mcp/pkg/evidence"
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
	Error    string      `json:"error"`
	FailedAt interface{} `json:"failed_at,omitempty"`
}

type manifest struct {
	Format       string `json:"format"`
	CreatedAt    string `json:"created_at"`
	EvidenceFile string `json:"evidence_file"`
	Records      int    `json:"records"`
	LastHash     string `json:"last_hash"`
	PolicyRef    string `json:"policy_ref"`
	Notes        string `json:"notes"`

	PolicyFileSHA256 string `json:"policy_file_sha256,omitempty"`
	DataFileSHA256   string `json:"data_file_sha256,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evidra-evidence <verify|export> [flags]")
		return exitInputError
	}

	switch args[0] {
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: evidra-evidence <verify|export> [flags]")
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
		_ = writeJSON(stdout, verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		})
		return exitVerifyFailed
	}

	meta, err := evidence.MetadataAtPath(*evidencePath)
	if err != nil {
		_ = writeJSON(stdout, verifyFailOutput{
			OK:    false,
			Error: err.Error(),
		})
		return exitVerifyFailed
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

	m := manifest{
		Format:       "evidra-audit-pack-v0.1",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		EvidenceFile: "evidence/evidence.log",
		Records:      meta.Records,
		LastHash:     meta.LastHash,
		PolicyRef:    meta.PolicyRef,
		Notes:        "Evidra audit pack v0.1",
	}
	if len(policyBytes) > 0 {
		m.PolicyFileSHA256 = sha256Hex(policyBytes)
	}
	if len(dataBytes) > 0 {
		m.DataFileSHA256 = sha256Hex(dataBytes)
	}

	if err := writeAuditPack(*outPath, *evidencePath, policyBytes, dataBytes, m); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitExportFailure
	}
	_ = writeJSON(stdout, map[string]interface{}{
		"ok":  true,
		"out": *outPath,
	})
	return exitOK
}

func writeAuditPack(outPath, evidencePath string, policyBytes []byte, dataBytes []byte, m manifest) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := addFileToTar(tw, evidencePath, "evidence/evidence.log"); err != nil {
		return err
	}

	manifestBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := addBytesToTar(tw, "manifest.json", manifestBytes); err != nil {
		return err
	}

	if len(policyBytes) > 0 {
		if err := addBytesToTar(tw, "policy/policy.rego", policyBytes); err != nil {
			return err
		}
	}
	if len(dataBytes) > 0 {
		if err := addBytesToTar(tw, "policy/data.json", dataBytes); err != nil {
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
