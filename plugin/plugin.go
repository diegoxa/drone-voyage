// Copyright 2024 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// maxPushAttempts bounds how many times CommitAndPush is retried when
	// the remote branch moved in the meantime (another deploy raced us).
	maxPushAttempts = 4

	// baseRetryDelay/maxRetryDelay control the exponential backoff between
	// push retries: 2s, 4s, 8s, capped at maxRetryDelay.
	baseRetryDelay = 2 * time.Second
	maxRetryDelay  = 10 * time.Second
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	GithubRepo      string   `envconfig:"PLUGIN_GITHUB_REPO"`
	GithubSSHKey    string   `envconfig:"PLUGIN_GITHUB_SSH_KEY"`
	Image           string   `envconfig:"PLUGIN_IMAGE"`
	DeploymentFiles []string `envconfig:"PLUGIN_DEPLOYMENT_FILES"`
	ContainerNames  []string `envconfig:"PLUGIN_CONTAINER_NAMES"`
	CommitAuthor    string   `envconfig:"PLUGIN_COMMIT_AUTHOR"`
	CommitEmail     string   `envconfig:"PLUGIN_COMMIT_EMAIL"`
}

// Exec executes the plugin.
func Exec(ctx context.Context, args Args) error {
	logrus.Debugln("arguments:")
	logrus.Debugf("Github Repo: %s", args.GithubRepo)
	logrus.Debugf("Commit Author: %s <%s>", args.CommitAuthor, args.CommitEmail)

	logrus.Printf("Image to Deploy: %s", args.Image)
	logrus.Printf("Deployment Files: %s", args.DeploymentFiles)
	logrus.Printf("Container Names: %v", args.ContainerNames)

	repo, err := cloneRepo(
		args.GithubRepo,
		args.GithubSSHKey,
		args.CommitAuthor,
		args.CommitEmail,
	)
	if err != nil {
		return err
	}
	defer repo.Cleanup()

	return updateAndPush(repo, args)
}

// updateAndPush patches the manifests and pushes the result, retrying with
// a fresh sync of the remote whenever the push is rejected because another
// deploy landed first. Each retry reapplies the same edit on top of the
// latest remote state instead of trying to merge or rebase shallow
// history, which go-git can't do reliably.
func updateAndPush(repo *Repo, args Args) error {
	for attempt := 1; ; attempt++ {
		updated, err := UpdateImage(repo, args.DeploymentFiles, args.Image, args.ContainerNames)
		if err != nil {
			return err
		}

		if !updated {
			logrus.Println("no deployment files were updated")
			return nil
		}

		err = repo.CommitAndPush()
		if err == nil {
			logrus.Println("deployment file(s) updated")
			return nil
		}

		if !IsRemoteAheadErr(err) || attempt >= maxPushAttempts {
			return fmt.Errorf("failed to push changes: %w", err)
		}

		delay := backoffDelay(attempt)
		logrus.Warnf("remote was updated concurrently, retrying (%d/%d) in %s: %v", attempt, maxPushAttempts-1, delay, err)
		time.Sleep(delay)

		if err := repo.SyncWithRemote(); err != nil {
			return fmt.Errorf("failed to sync with remote before retry: %w", err)
		}
	}
}

func backoffDelay(attempt int) time.Duration {
	delay := baseRetryDelay * time.Duration(1<<uint(attempt-1))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

func cloneRepo(gitRepo, gitSSHKey, commitAuthor, commitEmail string) (*Repo, error) {
	repo := NewGitRepo(
		gitRepo,
		commitAuthor,
		commitEmail,
		gitSSHKey,
	)
	if err := repo.Clone(); err != nil {
		return nil, err
	}
	return repo, nil
}
