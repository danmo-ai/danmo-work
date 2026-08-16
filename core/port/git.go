package port

// GitCredentialProvider materializes git credentials for sandboxed execution.
// Container backends bind-mount the returned host directory (containing a
// derived .git-credentials file) read-only into project containers so that
// agent exec_shell git commands can authenticate. Return "" when unavailable.
type GitCredentialProvider interface {
	CredentialDir() string
}
