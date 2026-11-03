package archive

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
)

func TestEncryptWriterPersistsOnlyCiphertext(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("private database fixture row")
	var ciphertext bytes.Buffer
	writer, err := NewEncryptWriter(&ciphertext, []string{identity.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(plaintext); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext.Bytes(), plaintext) {
		t.Fatal("plaintext persisted in encrypted output")
	}
	reader, err := age.Decrypt(bytes.NewReader(ciphertext.Bytes()), identity)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatal("decrypted archive differs from input")
	}
	wrongIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := age.Decrypt(bytes.NewReader(ciphertext.Bytes()), wrongIdentity); err == nil {
		t.Fatal("unrelated identity decrypted ciphertext")
	}
}
