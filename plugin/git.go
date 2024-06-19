package plugin

import (
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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

func (project *Repo) Clone() {
	logrus.Debugf("temp dir: %s\n", project.localDir)
	project.Cleanup()

	signer, err := ssh.NewPublicKeys("git", []byte(project.sshKey), "")
	if err != nil {
		logrus.Fatal(err)
	}

	project.repository, err = git.PlainClone(project.localDir, false, &git.CloneOptions{
		URL:      project.RepoGit,
		Progress: os.Stdout,
		Depth:    1,
		Auth:     signer,
	})
	if err != nil {
		logrus.Fatal(err)
	}
}

func (project *Repo) CommitAndPush() {
	logrus.Println("commit and push")

	worktree, err := project.repository.Worktree()
	if err != nil {
		logrus.Fatal(err)
	}

	_, err = worktree.Add(".")
	if err != nil {
		logrus.Fatal(err)
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
		logrus.Fatal(err)
	}

	err = project.repository.Push(&git.PushOptions{})
	if err != nil {
		logrus.Fatal(err)
	}
}

func (project *Repo) GetLocalDir() string {
	return project.localDir
}

func (project *Repo) Cleanup() {
	_ = os.RemoveAll(project.localDir)
}
