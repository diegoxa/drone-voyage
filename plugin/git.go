package plugin

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sirupsen/logrus"
)

type Repo struct {
	RepoGit        string
	localDir       string
	sshKey         string
	CommitterName  string
	CommitterEmail string
	repository     *git.Repository
	branch         string
	auth           transport.AuthMethod
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func NewGitRepo(gitAddress, committerName, committerEmail, sshKey string) *Repo {
	// in case key is formatted as single line with \n breaks
	key := strings.Replace(sshKey, "\\n", "\n", -1)

	return &Repo{
		RepoGit:        gitAddress,
		CommitterName:  committerName,
		CommitterEmail: committerEmail,
		localDir:       "/tmp/" + generateRandomString(6),
		sshKey:         key,
	}
}

func generateRandomString(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

// Clone fetches only what the plugin needs: the latest state of the
// default branch, with no history and no tags. A shallow (depth 1),
// single-branch clone is enough since the plugin only ever reads and
// rewrites the current version of a handful of manifest files.
func (project *Repo) Clone() error {
	logrus.Debugf("temp dir: %s\n", project.localDir)
	project.Cleanup()

	auth, err := buildAuth(project.sshKey)
	if err != nil {
		return err
	}
	project.auth = auth

	project.repository, err = git.PlainClone(project.localDir, false, &git.CloneOptions{
		URL:          project.RepoGit,
		Progress:     progressWriter(),
		Depth:        1,
		SingleBranch: true,
		Tags:         git.NoTags,
		Auth:         project.auth,
	})
	if err != nil {
		return fmt.Errorf("failed to clone %s: %w", project.RepoGit, err)
	}

	head, err := project.repository.Head()
	if err != nil {
		return fmt.Errorf("failed to resolve HEAD: %w", err)
	}
	project.branch = head.Name().Short()

	return nil
}

// progressWriter enables go-git's clone progress (server-side "Counting
// objects"/"Compressing objects" messages) only at debug/trace level. CI
// logs aren't a TTY, so the carriage returns that let a terminal overwrite
// one progress line instead print one line per percentage tick, flooding a
// normal run's logs; debug/trace runs still get the detail.
func progressWriter() io.Writer {
	if logrus.GetLevel() >= logrus.DebugLevel {
		return os.Stdout
	}
	return nil
}

// buildAuth builds SSH key auth when a key is provided. An empty key means
// no auth is configured, which is what a local filesystem remote (used in
// tests) or an already-authenticated URL needs.
func buildAuth(sshKey string) (transport.AuthMethod, error) {
	if sshKey == "" {
		return nil, nil
	}

	keys, err := ssh.NewPublicKeys("git", []byte(sshKey), "")
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh key: %w", err)
	}
	return keys, nil
}

func (project *Repo) CommitAndPush() error {
	logrus.Println("commit and push")

	worktree, err := project.repository.Worktree()
	if err != nil {
		return fmt.Errorf("failed to open worktree: %w", err)
	}

	if _, err := worktree.Add("."); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	commitMsg := "update image"
	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  project.CommitterName,
			Email: project.CommitterEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if err := project.repository.Push(&git.PushOptions{Auth: project.auth}); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}

// IsRemoteAheadErr reports whether err is a push rejection caused by the
// remote branch having moved since the repo was cloned or last synced,
// i.e. someone else pushed in the meantime. This is the only failure
// CommitAndPush retries are meant to recover from.
func IsRemoteAheadErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "non-fast-forward")
}

// SyncWithRemote fetches the latest commit on the tracked branch and hard
// resets the working copy to it, discarding the local commit left behind
// by a rejected CommitAndPush. Shallow history makes a real merge/rebase
// unreliable, so instead of reconciling the two histories the plugin just
// rebases its retry on whatever is on the remote right now: fetch, reset,
// then reapply the same image edit and commit again.
func (project *Repo) SyncWithRemote() error {
	// Clone was made with SingleBranch, which go-git tracks against
	// refs/remotes/origin/HEAD rather than the branch name. Fetch with an
	// explicit refspec instead of relying on that default, so the branch's
	// remote-tracking ref is always the one that gets updated here.
	remoteTrackingRef := plumbing.NewRemoteReferenceName("origin", project.branch)
	refSpec := config.RefSpec(fmt.Sprintf("+refs/heads/%s:%s", project.branch, remoteTrackingRef))

	err := project.repository.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       project.auth,
		Depth:      1,
		Force:      true,
		Tags:       git.NoTags,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to fetch latest: %w", err)
	}

	remoteRef, err := project.repository.Reference(remoteTrackingRef, true)
	if err != nil {
		return fmt.Errorf("failed to resolve origin/%s: %w", project.branch, err)
	}

	worktree, err := project.repository.Worktree()
	if err != nil {
		return fmt.Errorf("failed to open worktree: %w", err)
	}

	if err := worktree.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	}); err != nil {
		return fmt.Errorf("failed to reset to origin/%s: %w", project.branch, err)
	}

	return nil
}

func (project *Repo) GetLocalDir() string {
	return project.localDir
}

func (project *Repo) Cleanup() {
	_ = os.RemoveAll(project.localDir)
}
