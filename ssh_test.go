/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitkit

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestListenAndServe(t *testing.T) {
	tests := []struct {
		name       string
		serverFunc func(repo, keyDir string) *SSH
		err        bool
	}{
		{
			name: "default ssh server",
			serverFunc: func(repo, keyDir string) *SSH {
				return setupSSHServer(repo, keyDir)
			},
		},
		{
			name: "ssh server times out",
			serverFunc: func(repo, keyDir string) *SSH {
				server := setupSSHServer(repo, keyDir)
				timeout := time.Nanosecond * 1
				server.Timeout = &timeout
				return server
			},
			err: true,
		},
	}

	repo, err := createRepo()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(repo)
	keyDir, err := os.MkdirTemp("", "key-dir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(keyDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			server := tt.serverFunc(repo, keyDir)
			defer server.Stop()

			go func() {
				server.ListenAndServe(":2222")
			}()

			cloned, err := os.MkdirTemp("", "cloned")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(cloned)

			if err = retry(10, time.Second*1, func() error {
				_, err := net.Dial("tcp", "localhost:2222")
				return err
			}); err != nil {
				t.Fatal(err)
			}

			cmd := getCloneCommand(filepath.Base(repo), cloned)
			e := new(strings.Builder)
			cmd.Stderr = e
			err = cmd.Start()
			if err != nil {
				t.Fatalf("faile to start git clone command: %s", e.String())
			}

			err = cmd.Wait()
			g.Expect(err != nil).To(Equal(tt.err))
			_, err = os.Stat(filepath.Join(cloned, filepath.Base(repo)))
			if !tt.err {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}

}

func TestSshServerLatency(t *testing.T) {
	repo, err := createRepo()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(repo)
	keyDir, err := os.MkdirTemp("", "key-dir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(keyDir)

	server := setupSSHServer(repo, keyDir)
	latency := time.Second * 5
	server.Latency = &latency

	defer server.Stop()

	go func() {
		server.ListenAndServe(":2222")
	}()

	cloned, err := os.MkdirTemp("", "cloned")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cloned)

	if err = retry(10, time.Second*1, func() error {
		_, err := net.Dial("tcp", "localhost:2222")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	cmd := getCloneCommand(filepath.Base(repo), cloned)
	e := new(strings.Builder)
	cmd.Stderr = e

	err = cmd.Start()
	if err != nil {
		t.Fatalf("faile to start git clone command: %s", e.String())
	}

	g := NewWithT(t)
	start := time.Now()
	err = cmd.Wait()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(time.Now()).ShouldNot(BeTemporally("~", start, *server.Latency))
	_, err = os.Stat(filepath.Join(cloned, filepath.Base(repo)))
	g.Expect(err).ToNot(HaveOccurred())
}

func getCloneCommand(repoName, cmdDir string) *exec.Cmd {
	cmd := exec.Command("git", "clone", "ssh://git@localhost:2222/"+repoName)
	cmd.Dir = cmdDir
	cmd.Env = []string{"GIT_SSH_COMMAND=ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"}
	return cmd
}

func setupSSHServer(repo, keyDir string) *SSH {
	server := NewSSH(Config{
		Dir:    filepath.Dir(repo),
		KeyDir: keyDir,
	})

	server.PublicKeyLookupFunc = func(s string) (*PublicKey, error) {
		return &PublicKey{Id: "12345"}, nil
	}
	return server
}

func createRepo() (string, error) {
	repo, err := os.MkdirTemp("", "ssh-test")
	if err != nil {
		return "", err
	}

	// init git
	e := new(strings.Builder)
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	cmd.Stderr = e
	if _, err = cmd.Output(); err != nil {
		return "", fmt.Errorf("failed to initalize repo: %s", e.String())
	}
	e.Reset()

	if err = os.WriteFile(filepath.Join(repo, "homework"), []byte("all done"), 0644); err != nil {
		return "", err
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repo
	cmd.Stderr = e
	if _, err := cmd.Output(); err != nil {
		return "", fmt.Errorf("failed to add changes: %s", e.String())
	}
	e.Reset()

	cmd = exec.Command("git", "-c", "user.email=test@ssh.com", "-c", "user.name=test-user", "commit", "-m", "add homework")
	cmd.Dir = repo
	cmd.Stderr = e

	if _, err := cmd.Output(); err != nil {
		return "", fmt.Errorf("failed to commit changes: %s", e.String())
	}
	return repo, nil
}

func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f)
		}
		return err
	}

	return nil
}
