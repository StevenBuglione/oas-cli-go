package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const managedProcessRegistryName = "managed-mcp-pids.json"

type ProcessSupervisor struct {
	mu       sync.Mutex
	stateDir string
	pids     map[int]struct{}
}

type managedProcessRegistry struct {
	PIDs []int `json:"pids"`
}

func NewProcessSupervisor(stateDir string) *ProcessSupervisor {
	supervisor := &ProcessSupervisor{
		stateDir: stateDir,
		pids:     map[int]struct{}{},
	}
	_ = supervisor.load()
	return supervisor
}

func CleanupManagedProcesses(ctx context.Context, stateDir string) error {
	return NewProcessSupervisor(stateDir).Shutdown(ctx)
}

func (supervisor *ProcessSupervisor) Track(pid int) error {
	if pid <= 0 {
		return nil
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	supervisor.pids[pid] = struct{}{}
	return supervisor.persistLocked()
}

func (supervisor *ProcessSupervisor) Release(pid int) error {
	if pid <= 0 {
		return nil
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	delete(supervisor.pids, pid)
	return supervisor.persistLocked()
}

func (supervisor *ProcessSupervisor) Shutdown(ctx context.Context) error {
	supervisor.mu.Lock()
	pids := supervisor.snapshotLocked()
	supervisor.mu.Unlock()

	var errs []error
	for _, pid := range pids {
		if err := terminateProcess(ctx, pid); err != nil {
			errs = append(errs, err)
		}
	}

	supervisor.mu.Lock()
	supervisor.pids = map[int]struct{}{}
	persistErr := supervisor.persistLocked()
	supervisor.mu.Unlock()
	if persistErr != nil {
		errs = append(errs, persistErr)
	}
	return errors.Join(errs...)
}

func (supervisor *ProcessSupervisor) load() error {
	data, err := os.ReadFile(supervisor.registryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var registry managedProcessRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return err
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	for _, pid := range registry.PIDs {
		if pid > 0 {
			supervisor.pids[pid] = struct{}{}
		}
	}
	return nil
}

func (supervisor *ProcessSupervisor) registryPath() string {
	return filepath.Join(supervisor.stateDir, managedProcessRegistryName)
}

func (supervisor *ProcessSupervisor) snapshotLocked() []int {
	pids := make([]int, 0, len(supervisor.pids))
	for pid := range supervisor.pids {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

func (supervisor *ProcessSupervisor) persistLocked() error {
	if supervisor.stateDir == "" {
		return nil
	}
	path := supervisor.registryPath()
	if len(supervisor.pids) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(supervisor.stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(managedProcessRegistry{PIDs: supervisor.snapshotLocked()})
	if err != nil {
		return err
	}
	tempPath := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func terminateProcess(ctx context.Context, pid int) error {
	if !ProcessAlive(pid) {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return waitForProcessExit(ctx, pid)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	graceCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := waitForProcessExit(graceCtx, pid); err == nil {
		return nil
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return waitForProcessExit(ctx, pid)
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "linux" {
		if state, ok := linuxProcessState(pid); ok {
			return state != "Z"
		}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func linuxProcessState(pid int) (string, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "stat"))
	if err != nil {
		return "", false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return "", false
	}
	return fields[2], true
}

func waitForProcessExit(ctx context.Context, pid int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !ProcessAlive(pid) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
