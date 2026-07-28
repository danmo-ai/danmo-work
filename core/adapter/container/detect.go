package container

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"danmo-work/core/domain"
)

// Detect picks a Runtime. prefer may be auto|podman|docker|apple-container.
// Probe order when auto: on darwin prefer Apple `container`, then podman, docker;
// elsewhere podman then docker.
func Detect(prefer domain.EnvironmentEngine) (Runtime, error) {
	want := strings.ToLower(strings.TrimSpace(string(prefer)))
	switch want {
	case "", "auto":
		return detectAuto()
	case "podman":
		return requireCLI("podman", string(domain.EnvironmentEnginePodman))
	case "docker":
		return requireCLI("docker", string(domain.EnvironmentEngineDocker))
	case "apple-container", "apple", "container":
		return requireApple()
	default:
		return nil, fmt.Errorf("container: unknown engine %q (auto|podman|docker|apple-container)", prefer)
	}
}

func detectAuto() (Runtime, error) {
	var tried []string
	if runtime.GOOS == "darwin" {
		if r, err := tryApple(); err == nil {
			return r, nil
		} else {
			tried = append(tried, err.Error())
		}
	}
	for _, name := range []string{"podman", "docker"} {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return newCLIRuntime(p, name), nil
		}
		tried = append(tried, name+" not in PATH")
	}
	if runtime.GOOS != "darwin" {
		if r, err := tryApple(); err == nil {
			return r, nil
		} else {
			tried = append(tried, err.Error())
		}
	}
	return nil, fmt.Errorf("container: no engine found (%s)", strings.Join(tried, "; "))
}

func requireCLI(bin, name string) (Runtime, error) {
	p, err := exec.LookPath(bin)
	if err != nil || p == "" {
		return nil, fmt.Errorf("container: %s not in PATH", bin)
	}
	return newCLIRuntime(p, name), nil
}

func requireApple() (Runtime, error) {
	r, err := tryApple()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func tryApple() (Runtime, error) {
	p, err := exec.LookPath("container")
	if err != nil || p == "" {
		return nil, fmt.Errorf("apple-container: `container` not in PATH")
	}
	if !isAppleContainerCLI(p) {
		return nil, fmt.Errorf("apple-container: PATH `container` is not Apple container CLI")
	}
	return newAppleRuntime(p), nil
}
