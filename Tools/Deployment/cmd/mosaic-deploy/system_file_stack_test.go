package main

// system_file_stack_test.go verifies, at the composition root, that the protocol and the
// bundle come from the file the catalog stack resolved: a stack with no supplier stops the
// run before anything is written and names the missing file, with no fallback to the MOSAIC
// root's default catalog (which is complete on disk in every case here).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog/stack/systemfile"
)

// systemFileNames are the two files a user layer can lack.
var systemFileNames = []string{"CommunicationProtocol.md", "DeployedSections.md"}

func TestBinary_UserLayerLacksSystemFile_StopsBeforeWritingAndNamesTheFile(t *testing.T) {
	for _, missing := range systemFileNames {
		t.Run(missing, func(t *testing.T) {
			mosaicRoot := newStackMosaicRoot(t)
			user := newStackUserCatalog(t, "user-a", "2.0.0")
			if err := os.Remove(filepath.Join(user, missing)); err != nil {
				t.Fatalf("remove %s from the user catalog: %v", missing, err)
			}
			ws := t.TempDir()

			run := runDeployBinary(t, "deploy", "--mosaic-root", mosaicRoot, "--harness", "claude-code",
				"--workspace", ws, "--workflows", "user-a-wf", "--auto-confirm", "--catalog-folder", user)

			if run.ExitCode == 0 {
				t.Errorf("exit code 0, want a failure: the stack supplies no %s and there is no fallback to the default catalog\nstdout: %s",
					missing, run.Stdout)
			}
			if !strings.Contains(run.Stdout+run.Stderr, missing) {
				t.Errorf("output does not name the missing file %s\nstdout: %s\nstderr: %s", missing, run.Stdout, run.Stderr)
			}
			if n := dirEntryCount(t, ws); n != 0 {
				t.Errorf("workspace holds %d entries after the failed run, want none written", n)
			}
		})
	}
}

func TestBinary_UserLayerSuppliesBothSystemFiles_DeploysWithoutTheDefaultCatalog(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	for _, name := range systemFileNames {
		if err := os.Remove(filepath.Join(mosaicRoot, "Catalog", name)); err != nil {
			t.Fatalf("remove %s from the default catalog: %v", name, err)
		}
	}
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	ws := t.TempDir()

	run := runDeployBinary(t, deployArgs(mosaicRoot, ws, "user-a-wf", "--catalog-folder", user)...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, want success: the user layer supplies both system files; stderr: %s", run.ExitCode, run.Stderr)
	}
}

func TestReloadCatalog_FolderLacksSystemFile_ServiceStopsWithNotSupplied(t *testing.T) {
	for _, missing := range systemFileNames {
		t.Run(missing, func(t *testing.T) {
			mosaicRoot := newStackMosaicRoot(t)
			deps := buildTestDeps(t, mosaicRoot, filepath.Join(mosaicRoot, "Catalog"))
			deps.Interaction = interactiontest.NewBuilder().Build()
			folder := newStackUserCatalog(t, "reload-user", "2.0.0")
			if err := os.Remove(filepath.Join(folder, missing)); err != nil {
				t.Fatalf("remove %s from the folder: %v", missing, err)
			}
			svc, err := buildReloadFunc(mosaicRoot, deps)(stackconfig.Description{UserLayers: []string{folder}}, stackconfig.OriginStackInput)
			if err != nil {
				t.Fatalf("reload(%q): %v", folder, err)
			}

			_, err = svc.RenderAgent(context.Background(), app.RenderAgentRequest{
				SourceAgentKey:  "reload-user-agent",
				TargetHarnessID: "claude-code",
				DestinationPath: filepath.Join(t.TempDir(), "out.md"),
				DryRun:          true,
			})

			var notSupplied *systemfile.NotSuppliedError
			if !errors.As(err, &notSupplied) {
				t.Fatalf("RenderAgent error = %v; want *systemfile.NotSuppliedError from the reloaded stack", err)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Errorf("error %q does not name %s", err.Error(), missing)
			}
		})
	}
}
