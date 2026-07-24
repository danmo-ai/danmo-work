//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"unsafe"

	"danmo-work/core/domain"

	"golang.org/x/sys/unix"
)

const (
	landlockAccessFSRead = unix.LANDLOCK_ACCESS_FS_EXECUTE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR

	landlockAccessFSWrite = unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
		unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
		unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
		unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
		unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM |
		unix.LANDLOCK_ACCESS_FS_REFER |
		unix.LANDLOCK_ACCESS_FS_TRUNCATE

	landlockAccessFSAll = landlockAccessFSRead | landlockAccessFSWrite
)

func landlockAvailable() bool {
	abi, err := landlockABIVersion()
	return err == nil && abi >= 1
}

func landlockABIVersion() (int, error) {
	r1, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0,
		0,
		uintptr(unix.LANDLOCK_CREATE_RULESET_VERSION),
	)
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

func applyLandlock(workdir string, mode domain.SandboxMode) error {
	abi, err := landlockABIVersion()
	if err != nil {
		return fmt.Errorf("ABI probe: %w", err)
	}
	handled := landlockAccessFSAll
	if abi < 2 {
		// REFER added in ABI 2
		handled &^= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi < 3 {
		handled &^= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}

	attr := unix.LandlockRulesetAttr{Access_fs: uint64(handled)}
	fd, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		unsafe.Sizeof(attr),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("create ruleset: %w", errno)
	}
	ruleset := int(fd)
	defer unix.Close(ruleset)

	// Read everywhere.
	if err := landlockAllowPath(ruleset, "/", landlockAccessFSRead&uint64(handled)); err != nil {
		return err
	}
	// Writable temps.
	for _, p := range []string{"/tmp", "/var/tmp", "/dev/shm"} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			_ = landlockAllowPath(ruleset, p, uint64(handled))
		}
	}
	if mode != domain.SandboxModeReadOnly {
		if err := landlockAllowPath(ruleset, workdir, uint64(handled)); err != nil {
			return fmt.Errorf("workdir rule: %w", err)
		}
	}

	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %w", err)
	}
	_, _, errno = unix.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		uintptr(ruleset),
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("restrict_self: %w", errno)
	}
	return nil
}

func landlockAllowPath(ruleset int, path string, access uint64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	attr := unix.LandlockPathBeneathAttr{
		Allowed_access: access,
		Parent_fd:      int32(f.Fd()),
	}
	_, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(ruleset),
		uintptr(unix.LANDLOCK_RULE_PATH_BENEATH),
		uintptr(unsafe.Pointer(&attr)),
	)
	if errno != 0 {
		return fmt.Errorf("add_rule %s: %w", path, errno)
	}
	return nil
}
