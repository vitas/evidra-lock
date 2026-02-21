package evidence

import pkgevidence "samebits.com/evidra-mcp/pkg/evidence"

// TODO(monorepo-split): move core/evidence model and storage to standalone core module.

type PolicyDecision = pkgevidence.PolicyDecision
type ExecutionResult = pkgevidence.ExecutionResult
type EvidenceRecord = pkgevidence.EvidenceRecord
type Record = pkgevidence.Record
type Store = pkgevidence.Store

type StoreError = pkgevidence.StoreError

const (
	ErrorCodeStoreBusy               = pkgevidence.ErrorCodeStoreBusy
	ErrorCodeLockNotSupportedWindows = pkgevidence.ErrorCodeLockNotSupportedWindows
)

var (
	NewStore         = pkgevidence.NewStore
	NewStoreWithPath = pkgevidence.NewStoreWithPath
	Append           = pkgevidence.Append
	ValidateChain    = pkgevidence.ValidateChain
	ComputeHash      = pkgevidence.ComputeHash
	IsStoreBusyError = pkgevidence.IsStoreBusyError
	ErrorCode        = pkgevidence.ErrorCode
)
