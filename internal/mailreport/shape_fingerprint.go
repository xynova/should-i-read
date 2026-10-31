package mailreport

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ShapeFingerprint returns a stable hex SHA-256 of domain and subject template.
func ShapeFingerprint(domain, template string) string {
	d := strings.TrimSpace(domain)
	tmpl := strings.TrimSpace(template)
	sum := sha256.Sum256([]byte(d + "\n" + tmpl))
	return hex.EncodeToString(sum[:])
}
