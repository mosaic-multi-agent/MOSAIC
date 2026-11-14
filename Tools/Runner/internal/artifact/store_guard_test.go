package artifact_test

// Tests for the store's write guard: once a read has reported damage, no writer
// changes the file until a later read succeeds, and the re-reading writers also
// refuse on their own damaged re-read. A refused write is an
// ErrWriteAfterDamage that carries the damage, and the file stays byte-for-byte
// as it was.

import (
	"context"
	"errors"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func requireRefusedAfterDamage(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("the write succeeded, want a refusal")
	}
	if !errors.Is(err, domain.ErrWriteAfterDamage) {
		t.Errorf("error %v does not match domain.ErrWriteAfterDamage", err)
	}
	var damage *domain.ArtifactDamageError
	if !errors.As(err, &damage) {
		t.Errorf("error %v carries no *domain.ArtifactDamageError", err)
	}
	if err == error(damage) {
		t.Error("a refused write returned the bare damage error")
	}
}

func TestWriters_AfterADamagedRead_RefuseAndLeaveTheFileUnchanged(t *testing.T) {
	for _, w := range guardWriters {
		for _, dmg := range guardDamages {
			t.Run(w.name+"/"+dmg.name, func(t *testing.T) {
				store, path, good := guardFixture(t)
				damaged := dmg.doc()
				writeFileT(t, path, damaged)
				if _, err := store.Read(context.Background()); err == nil {
					t.Fatal("the damaged fixture was read without error")
				}

				err := w.write(context.Background(), store, good)

				requireRefusedAfterDamage(t, err)
				if got := readFileT(t, path); got != damaged {
					t.Errorf("the refused write changed the file:\n%s", got)
				}
			})
		}
	}
}

func TestRereadingWriters_OnTheirOwnDamagedReread_RefuseWithoutAPriorRead(t *testing.T) {
	for _, w := range rereadingWriters() {
		for _, dmg := range guardDamages {
			t.Run(w.name+"/"+dmg.name, func(t *testing.T) {
				_, path, good := guardFixture(t)
				damaged := dmg.doc()
				writeFileT(t, path, damaged)
				fresh := artifact.NewFileStore(path)

				err := w.write(context.Background(), fresh, good)

				requireRefusedAfterDamage(t, err)
				if got := readFileT(t, path); got != damaged {
					t.Errorf("the refused write changed the file:\n%s", got)
				}
			})
		}
	}
}

func TestWriters_AfterTheFileIsFixedAndReadSucceeds_WritesWorkAgain(t *testing.T) {
	for _, w := range guardWriters {
		t.Run(w.name, func(t *testing.T) {
			store, path, good := guardFixture(t)
			valid := readFileT(t, path)
			writeFileT(t, path, guardDamages[1].doc())
			if _, err := store.Read(context.Background()); err == nil {
				t.Fatal("the damaged fixture was read without error")
			}
			writeFileT(t, path, valid)
			if _, err := store.Read(context.Background()); err != nil {
				t.Fatalf("Read of the fixed file: %v", err)
			}

			err := w.write(context.Background(), store, good)

			if err != nil {
				t.Fatalf("write after a successful read: %v", err)
			}
			if got := readFileT(t, path); got == valid {
				t.Error("the write did not change the file")
			}
		})
	}
}

func TestApply_WhileTheGuardIsSet_RefusesEvenThoughTheFileIsValidAgain(t *testing.T) {
	store, path, good := guardFixture(t)
	valid := readFileT(t, path)
	writeFileT(t, path, guardDamages[1].doc())
	if _, err := store.Read(context.Background()); err == nil {
		t.Fatal("the damaged fixture was read without error")
	}
	writeFileT(t, path, valid) // fixed behind the store's back; no read yet

	err := guardWriters[0].write(context.Background(), store, good)

	requireRefusedAfterDamage(t, err)
	if got := readFileT(t, path); got != valid {
		t.Errorf("the refused Apply changed the file:\n%s", got)
	}
}

func TestGuard_IsPerStoreInstance(t *testing.T) {
	store, path, good := guardFixture(t)
	other := artifact.NewFileStore(path)
	valid := readFileT(t, path)
	writeFileT(t, path, guardDamages[1].doc())
	if _, err := store.Read(context.Background()); err == nil {
		t.Fatal("the damaged fixture was read without error")
	}
	writeFileT(t, path, valid)

	err := guardWriters[0].write(context.Background(), other, good)

	if err != nil {
		t.Errorf("a store instance that never read damage refused Apply: %v", err)
	}
}

func TestRefusedWriteDoesNotClearTheGuard(t *testing.T) {
	store, path, good := guardFixture(t)
	writeFileT(t, path, guardDamages[0].doc())
	if _, err := store.Read(context.Background()); err == nil {
		t.Fatal("the damaged fixture was read without error")
	}

	first := guardWriters[0].write(context.Background(), store, good)
	second := guardWriters[0].write(context.Background(), store, good)

	requireRefusedAfterDamage(t, first)
	requireRefusedAfterDamage(t, second)
}
