// Copyright 2024 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"

	"github.com/sirupsen/logrus"
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
	ContainerName   string   `envconfig:"PLUGIN_CONTAINER_NAME"`
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
	logrus.Printf("Conatiner Name: %s", args.ContainerName)

	repo := cloneRepo(
		args.GithubRepo,
		args.GithubSSHKey,
		args.CommitAuthor,
		args.CommitEmail,
	)
	defer repo.Cleanup()

	done := UpdateImage(repo, args.DeploymentFiles, args.Image, args.ContainerName)

	if done {
		repo.CommitAndPush()
		logrus.Println("deployment file(s) updated")
		return nil
	}

	logrus.Println("no deployment files were updated")
	return nil
}

func cloneRepo(gitRepo, gitSSHKey, commitAuthor, commitEmail string) *Repo {
	repo := NewGitRepo(
		gitRepo,
		commitAuthor,
		commitEmail,
		gitSSHKey,
	)
	repo.Clone()
	return repo
}
