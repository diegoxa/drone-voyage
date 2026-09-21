// Copyright 2024 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"log"
	"os"
	"testing"
)

func TestPlugin(t *testing.T) {
	sshKey := os.Getenv("GIT_SSH_KEY")
	gitRepo := os.Getenv("GIT_REPO")

	repo, err := cloneRepo(
		gitRepo,
		sshKey,
		"Deployer",
		"email@email.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Cleanup()

	log.Println(repo.GetLocalDir())

	done, err := UpdateImage(repo, []string{"folder/deployment.yaml", "folder2/migration-job.yaml"}, "docker/test:1", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Log("no deployment files were updated")
	}
}
