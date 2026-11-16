//go:build unix

// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMainCancelsComponentOnSignal(t *testing.T) {
	for _, signal := range []os.Signal{syscall.SIGTERM, os.Interrupt} {
		t.Run(signal.String(), func(t *testing.T) {
			cmd, done, childPID := startMainComponent(t, false)
			if err := cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("dpm did not exit after signal")
			}
			if cmd.ProcessState.Success() {
				t.Fatal("interrupted component unexpectedly succeeded")
			}
			deadline := time.Now().Add(3 * time.Second)
			for syscall.Kill(childPID, 0) == nil {
				if time.Now().After(deadline) {
					t.Fatalf("component process %d survived dpm receiving %v", childPID, signal)
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Logf("dpm handled %v and component process %d exited", signal, childPID)
		})
	}
}

func TestMainPreservesComponentExitCode(t *testing.T) {
	cmd, done, _ := startMainComponent(t, true)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("dpm did not exit after component")
	}
	if got := cmd.ProcessState.ExitCode(); got != 7 {
		t.Fatalf("component exit code = %d, want 7", got)
	}
}

func TestMainComponentCleanupBeforePIDRead(t *testing.T) {
	process := startMainComponentProcess(t, false)
	deadline := time.Now().Add(5 * time.Second)
	for {
		// The component has started, but cleanup has never observed its PID.
		if info, err := os.Stat(process.pidFile); err == nil && info.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("component did not publish its PID marker")
		}
		time.Sleep(10 * time.Millisecond)
	}
	process.stop()
	data, err := os.ReadFile(process.pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid component PID %q", data)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("component process %d remained after startup cleanup: %v", pid, err)
	}
	t.Logf("startup cleanup reaped dpm and stopped component %d without reading its PID first", pid)
}

// Each process invokes either the real main function or a synthetic native
// component. The latter records its PID so the parent can verify and clean up
// the actual subprocess without an SDK or registry. Only fixture teardown uses
// the private process group; signal assertions target the DPM process alone.
func TestDPMMainProcessHelper(t *testing.T) {
	switch os.Args[len(os.Args)-1] {
	case "dpm-process":
		os.Args = []string{"dpm", "signal-probe"}
		main()
		os.Exit(0)
	case "component-process":
		pidFile := os.Getenv("DPM_TEST_PID_FILE")
		if err := os.WriteFile(pidFile+".tmp", []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(2)
		}
		if err := os.Rename(pidFile+".tmp", pidFile); err != nil {
			os.Exit(2)
		}
		if os.Getenv("DPM_TEST_EXIT") == "true" {
			os.Exit(7)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
}

type mainComponentProcess struct {
	cmd                 *exec.Cmd
	done                <-chan struct{}
	pidFile, outputPath string
	stop                func()
}

func startMainComponentProcess(t *testing.T, exit bool) *mainComponentProcess {
	t.Helper()
	dir := t.TempDir()
	componentDir := filepath.Join(dir, "component")
	if err := os.Mkdir(componentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(executable, filepath.Join(componentDir, "probe")); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"component/component.yaml": `apiVersion: digitalasset.com/v1
kind: Component
spec:
  commands:
    - name: signal-probe
      path: ./probe
      exec-args: ["-test.run=^TestDPMMainProcessHelper$", "--", "component-process"]
`,
		"sdk-manifest.yaml": `apiVersion: digitalasset.com/v1
kind: SdkManifest
spec:
  version: 0.0.0
  edition: open-source
  components:
    probe:
      local-path: ./component
`,
		"daml.yaml":          "name: signal-test\nversion: 0.0.0\nsource: .\ndependencies: []\n",
		"registry-auth.json": "{}",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outputPath := filepath.Join(dir, "output")
	output, err := os.Create(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	pidFile := filepath.Join(dir, "component.pid")
	cmd := exec.Command(executable, "-test.run=^TestDPMMainProcessHelper$", "--", "dpm-process")
	// This group belongs only to the fixture. Assertions signal the DPM PID,
	// never the group; group signaling is reserved for unconditional teardown.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = output, output
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + os.TempDir(),
		"DPM_HOME=" + filepath.Join(dir, "dpm-home"),
		"DPM_ASSEMBLY=" + filepath.Join(dir, "sdk-manifest.yaml"),
		"DPM_REGISTRY_AUTH=" + filepath.Join(dir, "registry-auth.json"),
		"DPM_AUTO_INSTALL=false",
		"DAML_PACKAGE=" + dir,
		"DPM_TEST_PID_FILE=" + pidFile,
		fmt.Sprintf("DPM_TEST_EXIT=%t", exit),
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	groupID := cmd.Process.Pid
	var once sync.Once
	stop := func() {
		once.Do(func() {
			// The child may not yet have published its marker. Kill only the
			// recorded private group, without trusting a PID read from a file.
			if err := syscall.Kill(-groupID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				t.Errorf("failed to stop fixture process group %d: %v", groupID, err)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("failed to reap dpm helper")
			}
			deadline := time.Now().Add(3 * time.Second)
			for syscall.Kill(-groupID, 0) == nil {
				if time.Now().After(deadline) {
					t.Errorf("fixture process group %d remained after cleanup", groupID)
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
	t.Cleanup(stop)
	return &mainComponentProcess{cmd, done, pidFile, outputPath, stop}
}

func startMainComponent(t *testing.T, exit bool) (*exec.Cmd, <-chan struct{}, int) {
	t.Helper()
	process := startMainComponentProcess(t, exit)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if data, err := os.ReadFile(process.pidFile); err == nil {
			childPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil || childPID <= 0 {
				t.Fatalf("invalid component PID %q", data)
			}
			return process.cmd, process.done, childPID
		}
		select {
		case <-process.done:
			data, _ := os.ReadFile(process.outputPath)
			t.Fatalf("dpm exited before component started: %s", data)
		default:
		}
		if time.Now().After(deadline) {
			data, _ := os.ReadFile(process.outputPath)
			t.Fatalf("component did not start: %s", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
