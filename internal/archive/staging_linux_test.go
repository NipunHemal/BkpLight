package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"bkplight.local/bkplight/internal/safeerr"
	"filippo.io/age"
)

const fixtureID = "0123456789abcdef0123456789abcdef"

func recipient(t *testing.T) (*age.X25519Identity, string) {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return identity, identity.Recipient().String()
}

func TestStagerFinalizesOnlyEncryptedArtifact(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	identity, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("fixture database content")
	if _, err := session.Write(plain); err != nil {
		t.Fatal(err)
	}
	artifact, err := session.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	if artifact.RelativePath != fixtureID+".age" || artifact.CiphertextSize <= 0 || len(artifact.CiphertextSHA256) != 64 {
		t.Fatalf("invalid artifact metadata: %+v", artifact)
	}
	if err := stager.Verify(artifact.RelativePath, artifact.CiphertextSize, artifact.CiphertextSHA256); err != nil {
		t.Fatalf("finalized artifact failed verification: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".partial")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial remains after finalize: %v", err)
	}
	ciphertext, err := os.ReadFile(filepath.Join(dir, artifact.RelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, plain) {
		t.Fatal("plaintext persisted on disk")
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("staging directory not private: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, artifact.RelativePath)); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("ciphertext file not private: %v", err)
	}
}

func TestStagerVerifyRejectsTamperedCiphertext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := session.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, artifact.RelativePath), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stager.Verify(artifact.RelativePath, artifact.CiphertextSize, artifact.CiphertextSHA256); err == nil {
		t.Fatal("tampered ciphertext passed hash verification")
	}
}

func TestStagerRejectsCapAndLeavesNoValidArtifact(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 256})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err == nil {
		_, err = session.Write([]byte(strings.Repeat("x", 1024)))
		if err == nil {
			_, err = session.Finalize()
		}
		_ = session.Abort()
	}
	if err == nil {
		t.Fatal("over-cap artifact succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".age")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("over-cap artifact finalized: %v", err)
	}
}

func TestStagerReservationsAreAggregate(t *testing.T) {
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: filepath.Join(t.TempDir(), "staging"), MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	a, err := stager.Begin(context.Background(), fixtureID, []string{key}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Abort()
	_, err = stager.Begin(context.Background(), "abcdef0123456789abcdef0123456789", []string{key}, 3000)
	if err == nil || safeerr.Public(err).Code != safeerr.Busy {
		t.Fatalf("second reservation did not respect aggregate cap: %v", err)
	}
}

func TestStagerConcurrentWritersStayWithinCap(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	ids := []string{fixtureID, "abcdef0123456789abcdef0123456789"}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			session, err := stager.Begin(context.Background(), id, []string{key}, 0)
			if err != nil {
				return
			}
			if _, err = session.Write([]byte(strings.Repeat("x", 2048))); err == nil {
				_, err = session.Finalize()
			}
			if err != nil {
				_ = session.Abort()
			}
		}(id)
	}
	wg.Wait()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	if total > 2048 {
		t.Fatalf("parallel writers exceeded cap: %d", total)
	}
}

func TestStagerRejectsSymlinkDirectory(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "linked")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStager(Options{Directory: link, MaxBytes: 4096}); err == nil {
		_ = s.Close()
		t.Fatal("symlink staging path accepted")
	}
}

func TestStagerRejectsSymlinkAncestor(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	if err := os.MkdirAll(filepath.Join(actual, "staging"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "linked")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStager(Options{Directory: filepath.Join(link, "staging"), MaxBytes: 4096}); err == nil {
		_ = s.Close()
		t.Fatal("symlink ancestor accepted")
	}
}

func TestStagerAncestorSubstitutionDoesNotMutateReplacement(t *testing.T) {
	parent := t.TempDir()
	original := filepath.Join(parent, "ancestor")
	replacement := filepath.Join(parent, "replacement")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(replacement, "staging"), 0700); err != nil {
		t.Fatal(err)
	}
	var swapErr error
	opened, err := acquireDirectoryAt(filepath.Join(original, "staging"), func(part string) {
		if part != "ancestor" || swapErr != nil {
			return
		}
		if swapErr = os.Rename(original, filepath.Join(parent, "moved")); swapErr == nil {
			swapErr = os.Symlink(replacement, original)
		}
	})
	if opened != nil {
		_ = opened.Close()
	}
	if swapErr != nil {
		t.Fatal(swapErr)
	}
	if err == nil {
		t.Fatal("substituted ancestor was accepted")
	}
	entries, err := os.ReadDir(filepath.Join(replacement, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("replacement tree was mutated: %v, %d entries", err, len(entries))
	}
}

func TestStagerRefusesExistingFinalArtifactID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	first, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Finalize(); err != nil {
		t.Fatal(err)
	}
	second, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err == nil {
		_ = second.Abort()
		t.Fatal("existing final artifact ID was reused")
	}
}

func TestStagerRecountsRetainedCiphertextAfterRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	first, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session, err := first.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := session.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenStager(Options{Directory: dir, MaxBytes: artifact.CiphertextSize - 1}); err == nil {
		_ = reopened.Close()
		t.Fatal("retained ciphertext was not counted toward cap")
	}
}

func TestStagerPreservesRenamedFileWhenDirectorySyncFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Write([]byte("fixture")); err != nil {
		t.Fatal(err)
	}
	stager.syncDirectory = func() error { return errors.New("synthetic sync failure") }
	if _, err := session.Finalize(); err == nil {
		t.Fatal("failed directory sync reported success")
	}
	_ = session.Abort()
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".age")); err != nil {
		t.Fatalf("renamed ciphertext was lost before reconciliation: %v", err)
	}
	stager.syncDirectory = stager.dir.Sync
	if err := stager.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20}); err != nil {
		t.Fatalf("renamed file cannot be reconciled: %v", err)
	} else {
		_ = reopened.Close()
	}
}

func TestStagerAbortRemovesPartialAndReleasesBudget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	first, err := stager.Begin(context.Background(), fixtureID, []string{key}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Abort(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".partial")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aborted partial remains: %v", err)
	}
	second, err := stager.Begin(context.Background(), fixtureID, []string{key}, 3000)
	if err != nil {
		t.Fatalf("aborted reservation not released: %v", err)
	}
	if err := second.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestStagerHonorsFreeSpaceReserve(t *testing.T) {
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: filepath.Join(t.TempDir(), "staging"), MaxBytes: 1 << 20, MinFreeBytes: 1 << 62})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	if session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0); err == nil {
		_ = session.Abort()
		t.Fatal("free-space reserve was ignored")
	}
}

func TestStagerRejectsInvalidRecipientWithoutPartial(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	if session, err := stager.Begin(context.Background(), fixtureID, []string{"not-an-age-recipient"}, 0); err == nil {
		_ = session.Abort()
		t.Fatal("invalid recipient was accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".partial")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid recipient left partial: %v", err)
	}
}

func TestStagerCancelledWriterCannotFinalize(t *testing.T) {
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: filepath.Join(t.TempDir(), "staging"), MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	ctx, cancel := context.WithCancel(context.Background())
	session, err := stager.Begin(ctx, fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := session.Write([]byte("must fail")); err == nil {
		t.Fatal("cancelled writer accepted plaintext")
	}
	if _, err := session.Finalize(); err == nil {
		t.Fatal("cancelled writer finalized artifact")
	}
	if err := session.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestStagerReconcilesInterruptedPartialAndReclaimsBudget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fixtureID+".partial"), []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	if err := stager.ReconcilePartials(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".partial")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted partial remains: %v", err)
	}
	if stager.used != 0 {
		t.Fatalf("partial budget remains reserved: %d", stager.used)
	}
}

func TestStagerQuarantinesUncatalogedFinalCiphertext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	session, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := session.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := stager.QuarantineUncataloged(map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fixtureID+".orphan")); err != nil {
		t.Fatalf("orphaned ciphertext was lost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, artifact.RelativePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uncataloged ciphertext remains eligible: %v", err)
	}
	if err := stager.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenStager(Options{Directory: dir, MaxBytes: 4096}); err != nil {
		t.Fatalf("quarantine cannot be reopened: %v", err)
	} else {
		_ = reopened.Close()
	}
}
