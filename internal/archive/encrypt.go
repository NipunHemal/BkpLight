// Package archive creates encrypted, bounded local backup artifacts.
package archive

import (
	"io"

	"bkplight.local/bkplight/internal/safeerr"
	"filippo.io/age"
)

// NewEncryptWriter streams age ciphertext into dst. Only X25519 recipients are
// accepted; key material is never embedded in an error returned to callers.
func NewEncryptWriter(dst io.Writer, recipientStrings []string) (io.WriteCloser, error) {
	if dst == nil || len(recipientStrings) == 0 {
		return nil, &safeerr.Error{Code: safeerr.Invalid, Field: "encryption.recipients"}
	}
	recipients := make([]age.Recipient, 0, len(recipientStrings))
	for _, value := range recipientStrings {
		recipient, err := age.ParseX25519Recipient(value)
		if err != nil {
			return nil, &safeerr.Error{Code: safeerr.Invalid, Field: "encryption.recipients"}
		}
		recipients = append(recipients, recipient)
	}
	writer, err := age.Encrypt(dst, recipients...)
	if err != nil {
		return nil, &safeerr.Error{Code: safeerr.Unavailable, Field: "encryption"}
	}
	return writer, nil
}
