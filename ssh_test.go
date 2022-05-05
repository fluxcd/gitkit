package gitkit

import (
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
				server := NewSSH(Config{
					Dir:    filepath.Dir(repo),
					KeyDir: keyDir,
				})

				server.PublicKeyLookupFunc = func(s string) (*PublicKey, error) {
					return &PublicKey{Id: "12345"}, nil
				}
				return server
			},
		},
		{
			name: "ssh server times out",
			serverFunc: func(repo, keyDir string) *SSH {
				server := NewSSH(Config{
					Dir:    filepath.Dir(repo),
					KeyDir: keyDir,
				})

				server.PublicKeyLookupFunc = func(s string) (*PublicKey, error) {
					return &PublicKey{Id: "12345"}, nil
				}
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

			cmd := exec.Command("git", "clone", "ssh://git@localhost:2222/"+filepath.Base(repo))
			cmd.Dir = cloned
			cmd.Env = []string{"GIT_SSH_COMMAND=ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"}

			e := new(strings.Builder)
			cmd.Stderr = e
			err = cmd.Start()
			if err != nil {
				panic(err)
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

func createRepo() (string, error) {
	repo, err := os.MkdirTemp("", "ssh-test")
	if err != nil {
		return "", err
	}

	// init git
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if _, err = cmd.Output(); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(repo, "homework"), []byte("all done"), 0644); err != nil {
		return "", err
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repo

	if _, err := cmd.Output(); err != nil {
		return "", err
	}

	cmd = exec.Command("git", "commit", "-m", "add homework")
	cmd.Dir = repo

	if _, err := cmd.Output(); err != nil {
		return "", err
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
