// Copyright 2024 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"sigs.k8s.io/yaml"
)

// newBareRemote creates a local bare git repository seeded with a single
// deployment manifest, and returns its filesystem path. go-git recognizes
// a plain filesystem path as a "file" transport remote (shelling out to
// the local git binary), so cloning, fetching and pushing against it
// exercise the exact same code paths as a real SSH remote, without ever
// needing SSH credentials.
func newBareRemote(t *testing.T, seedFile, seedContent string) string {
	t.Helper()

	root := t.TempDir()
	bareDir := filepath.Join(root, "remote.git")
	if _, err := git.PlainInit(bareDir, true); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}

	seedDir := filepath.Join(root, "seed")
	seedRepo, err := git.PlainInit(seedDir, false)
	if err != nil {
		t.Fatalf("failed to init seed clone: %v", err)
	}

	if _, err := seedRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareDir},
	}); err != nil {
		t.Fatalf("failed to add origin to seed clone: %v", err)
	}

	if err := writeAndCommit(t, seedRepo, seedDir, seedFile, seedContent, "seed"); err != nil {
		t.Fatalf("failed to seed initial commit: %v", err)
	}

	head, err := seedRepo.Head()
	if err != nil {
		t.Fatalf("failed to resolve seed HEAD: %v", err)
	}

	refSpec := config.RefSpec(fmt.Sprintf("%s:%s", head.Name(), head.Name()))
	if err := seedRepo.Push(&git.PushOptions{RefSpecs: []config.RefSpec{refSpec}}); err != nil {
		t.Fatalf("failed to push seed commit to bare remote: %v", err)
	}

	return bareDir
}

// writeAndCommit writes content to path inside repoDir and commits it to
// repo's worktree.
func writeAndCommit(t *testing.T, repo *git.Repository, repoDir, path, content, message string) error {
	t.Helper()

	fullPath := filepath.Join(repoDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := worktree.Add(path); err != nil {
		return err
	}
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: "Seeder", Email: "seed@example.com", When: time.Now()},
	})
	return err
}

// readImage reads a freshly cloned copy of the remote's deployment file and
// returns the container image it holds, so tests can assert on what
// actually landed on the "remote" without depending on any local state.
func readImage(t *testing.T, remote, path string) string {
	t.Helper()

	checkDir := filepath.Join(t.TempDir(), "check")
	if _, err := git.PlainClone(checkDir, false, &git.CloneOptions{URL: remote}); err != nil {
		t.Fatalf("failed to clone remote for verification: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(checkDir, path))
	if err != nil {
		t.Fatalf("failed to read %s from remote: %v", path, err)
	}

	var m DeploymentManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	if len(m.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("%s has no containers", path)
	}
	return m.Spec.Template.Spec.Containers[0].Image
}

func TestCloneUpdateCommitPush(t *testing.T) {
	remote := newBareRemote(t, "deployment.yaml", deploymentYaml)

	repo, err := cloneRepo(remote, "", "Deployer", "deployer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Cleanup()

	updated, err := UpdateImage(repo, []string{"deployment.yaml"}, "new-image:v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected the manifest to be updated")
	}

	if err := repo.CommitAndPush(); err != nil {
		t.Fatalf("CommitAndPush failed: %v", err)
	}

	if got := readImage(t, remote, "deployment.yaml"); got != "new-image:v1" {
		t.Errorf("remote image = %q, want %q", got, "new-image:v1")
	}
}

func TestUpdateAndPushRetriesWhenRemoteMovedFirst(t *testing.T) {
	remote := newBareRemote(t, "deployment.yaml", deploymentYaml)

	repo, err := cloneRepo(remote, "", "Deployer", "deployer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Cleanup()

	// Simulate a second, concurrent deploy landing on the remote branch
	// after our clone but before our push: an independent clone commits
	// and pushes first, so our local HEAD is now behind origin.
	other, err := cloneRepo(remote, "", "Other Deployer", "other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Cleanup()

	if _, err := UpdateImage(other, []string{"deployment.yaml"}, "someone-else:v1", nil); err != nil {
		t.Fatal(err)
	}
	if err := other.CommitAndPush(); err != nil {
		t.Fatalf("concurrent push failed: %v", err)
	}

	// Our push should now be rejected once (non-fast-forward), then
	// succeed after updateAndPush syncs with the new remote tip and
	// reapplies the edit.
	args := Args{
		DeploymentFiles: []string{"deployment.yaml"},
		Image:           "new-image:v2",
	}
	if err := updateAndPush(repo, args); err != nil {
		t.Fatalf("updateAndPush failed to recover from the race: %v", err)
	}

	if got := readImage(t, remote, "deployment.yaml"); got != "new-image:v2" {
		t.Errorf("remote image = %q, want %q", got, "new-image:v2")
	}
}

func TestIsRemoteAheadErr(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("failed to push: non-fast-forward update: refs/heads/main"), true},
		{fmt.Errorf("failed to push: authentication required"), false},
	}

	for _, c := range cases {
		if got := IsRemoteAheadErr(c.err); got != c.want {
			t.Errorf("IsRemoteAheadErr(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
