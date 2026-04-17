package widget

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const dockerRefreshInterval = 15 * time.Second

// DockerWidget displays the number of running Docker containers.
type DockerWidget struct {
	Count    int
	Available bool // docker command exists
	lastRead time.Time
}

func (w *DockerWidget) Name() string { return "docker" }

func (w *DockerWidget) Render() string {
	if !w.Available {
		return ""
	}
	return fmt.Sprintf("\xf3\xb0\xa1\xa8 %d", w.Count) // 󰡨 nf-md-docker
}

func (w *DockerWidget) ColorLevel() string {
	if !w.Available || w.Count == 0 {
		return "green"
	}
	return ""
}

func (w *DockerWidget) NeedsRefresh() bool {
	return w.lastRead.IsZero() || time.Since(w.lastRead) >= dockerRefreshInterval
}

func (w *DockerWidget) MarkRefreshed() {
	w.lastRead = time.Now()
}

// ReadDockerCount returns the number of running containers.
// Returns -1 if docker is not available.
func ReadDockerCount() int {
	out, err := exec.Command("docker", "ps", "-q").Output()
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
