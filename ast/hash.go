// hash.go
package ast

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashContent returns a SHA256 hash of the given raw bytes.
func HashContent(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashNode returns a SHA256 hash of the stable binary representation of the AST node.
func HashNode(node Node) (string, error) {
	data, err := Encode(node)
	if err != nil {
		return "", err
	}
	return HashContent(data), nil
}
