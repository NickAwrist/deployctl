package git

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"deployctl/internal"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func CloneRepo(repoURL string, name string) (string, error) {
	if name == "" {
		name = repoNameFromURL(repoURL)
	}
	repoPath := filepath.Join(internal.GetRepositoryDirectory(), name)

	cloneURL, auth, err := cloneURLAndAuth(repoURL)
	if err != nil {
		return "", err
	}

	_, err = git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:  cloneURL,
		Auth: auth,
	})
	if err != nil {
		return "", cloneError(repoURL, err)
	}
	internal.Info("Repository cloned successfully into %s", repoPath)
	return repoPath, nil
}

func cloneURLAndAuth(repoURL string) (string, transport.AuthMethod, error) {
	if isSSHURL(repoURL) {
		return sshCloneURLAndAuth(repoURL)
	}

	if token := firstEnv("DEPLOYCTL_GIT_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GIT_TOKEN"); token != "" {
		username := firstEnv("DEPLOYCTL_GIT_USERNAME", "GITHUB_USERNAME", "GIT_USERNAME")
		if username == "" {
			username = "x-access-token"
		}
		return repoURL, &githttp.BasicAuth{Username: username, Password: token}, nil
	}

	return repoURL, nil, nil
}

func authForURL(repoURL string) (transport.AuthMethod, error) {
	_, auth, err := cloneURLAndAuth(repoURL)
	return auth, err
}

func sshCloneURLAndAuth(repoURL string) (string, transport.AuthMethod, error) {
	endpoint, err := transport.NewEndpoint(repoURL)
	if err != nil {
		return repoURL, nil, err
	}

	alias := endpoint.Host
	if hostName := ssh_config.Get(alias, "HostName"); hostName != "" {
		endpoint.Host = hostName
	}

	if endpoint.User == "" {
		endpoint.User = ssh_config.Get(alias, "User")
	}
	if endpoint.User == "" {
		endpoint.User = gitssh.DefaultUsername
	}

	if endpoint.Port == 0 || endpoint.Port == 22 {
		if port := ssh_config.Get(alias, "Port"); port != "" {
			if parsed, err := strconv.Atoi(port); err == nil {
				endpoint.Port = parsed
			}
		}
	}

	return endpoint.String(), sshAuthForEndpoint(endpoint.User, alias, endpoint.Host), nil
}

func sshAuthForEndpoint(user string, alias string, host string) transport.AuthMethod {
	return &gitssh.PublicKeysCallback{
		User: user,
		Callback: func() ([]ssh.Signer, error) {
			return sshSigners(user, alias, host)
		},
	}
}

func sshSigners(user string, alias string, host string) ([]ssh.Signer, error) {
	var signers []ssh.Signer

	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		if conn, err := net.Dial("unix", socket); err == nil {
			defer conn.Close()
			if agentSigners, err := agent.NewClient(conn).Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}

	for _, path := range sshIdentityFiles(user, alias, host) {
		keySigners, err := signersFromKeyFile(path)
		if err != nil {
			continue
		}
		signers = append(signers, keySigners...)
	}

	return uniqueSigners(signers), nil
}

func signersFromKeyFile(path string) ([]ssh.Signer, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if _, ok := err.(*ssh.PassphraseMissingError); ok {
		passphrase := firstEnv("DEPLOYCTL_SSH_KEY_PASSPHRASE", "SSH_KEY_PASSPHRASE")
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
		}
	}
	if err != nil {
		return nil, err
	}

	return []ssh.Signer{signer}, nil
}

func sshIdentityFiles(user string, alias string, host string) []string {
	configured := ssh_config.GetAll(alias, "IdentityFile")
	keyPath := firstEnv("DEPLOYCTL_SSH_KEY", "GIT_SSH_KEY", "SSH_KEY_PATH")

	paths := make([]string, 0, len(configured)+4)
	paths = append(paths, keyPath)
	for _, path := range configured {
		paths = append(paths, expandSSHPath(path, user, host))
	}
	paths = append(paths, defaultSSHKeyPaths()...)

	var existing []string
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}

	return existing
}

func uniqueSigners(signers []ssh.Signer) []ssh.Signer {
	seen := map[string]bool{}
	unique := make([]ssh.Signer, 0, len(signers))
	for _, signer := range signers {
		key := string(signer.PublicKey().Marshal())
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, signer)
	}
	return unique
}

func expandSSHPath(path string, remoteUser string, host string) string {
	path = strings.Trim(path, `"`)
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	if currentUser := os.Getenv("USER"); currentUser != "" {
		path = strings.ReplaceAll(path, "%u", currentUser)
	}
	path = strings.ReplaceAll(path, "%r", remoteUser)
	path = strings.ReplaceAll(path, "%h", host)
	path = os.ExpandEnv(path)

	return path
}

func defaultSSHKeyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
}

func isSSHURL(repoURL string) bool {
	repoURL = strings.TrimSpace(repoURL)
	if strings.HasPrefix(repoURL, "ssh://") {
		return true
	}

	return strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") && !strings.Contains(repoURL, "://")
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func cloneError(repoURL string, err error) error {
	if isSSHURL(repoURL) {
		return fmt.Errorf("%w\n\nFor private SSH repositories, deployctl uses your SSH agent, ~/.ssh/config IdentityFile entries, and standard private key paths. If authentication still fails, set DEPLOYCTL_SSH_KEY to the exact key used by ssh -T git@github.com.", err)
	}

	return fmt.Errorf("%w\n\nFor private HTTPS repositories, set DEPLOYCTL_GIT_TOKEN to a GitHub token with repository access.", err)
}

func repoNameFromURL(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	if i := strings.LastIndex(repoURL, ":"); i >= 0 {
		repoURL = repoURL[i+1:]
	}

	return filepath.Base(repoURL)
}
