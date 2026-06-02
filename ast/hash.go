// hash.go
package ast

import (
    "crypto/sha256"
    "encoding/hex"
)

// HashContent returns a SHA256 hash of the given JSON representation of an AST node.
func HashContent(data []byte) string {
    sum := sha256.Sum256(data)
    return hex.EncodeToString(sum[:])
}
