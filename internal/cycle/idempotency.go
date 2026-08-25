package cycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// OperationRecord persists the first result of an idempotent write operation.
// A repeated operation_id with the same normalized content returns the recorded
// result; different content returns OPERATION_CONTENT_CONFLICT.
type OperationRecord struct {
	OperationID string
	Digest      string
	Response    string
	AppliedAt   int64
}

// NormalizeContent produces a deterministic digest from any JSON-serializable
// content. It is used to compare repeated idempotent requests.
func NormalizeContent(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
