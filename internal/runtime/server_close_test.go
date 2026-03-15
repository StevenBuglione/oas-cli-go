package runtime

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestServerCloseWithContextTerminatesManagedProcesses(t *testing.T) {
	command := "sleep"
	args := []string{"30"}
	if runtime.GOOS == "windows" {
		command = "ping"
		args = []string{"127.0.0.1", "-n", "30"}
	}

	process := exec.Command(command, args...)
	if err := process.Start(); err != nil {
		t.Fatalf("start managed process: %v", err)
	}
	t.Cleanup(func() {
		if process.Process != nil {
			_ = process.Process.Kill()
		}
	})

	server := NewServer(Options{StateDir: t.TempDir()})
	if err := server.processSupervisor.Track(process.Process.Pid); err != nil {
		t.Fatalf("Track: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.CloseWithContext(ctx); err != nil {
		t.Fatalf("CloseWithContext: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- process.Wait()
	}()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatalf("managed process %d still alive after server close", process.Process.Pid)
	}
}
