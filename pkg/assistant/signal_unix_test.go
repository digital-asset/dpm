//go:build unix

// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// newTestAssistant returns an assistant whose streams go nowhere, so a
// component's output does not pollute the test log.
func newTestAssistant(t *testing.T, hook func(cmd *exec.Cmd)) *DamlAssistant {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { devNull.Close() })
	return &DamlAssistant{
		Stdin:         devNull,
		Stdout:        devNull,
		Stderr:        devNull,
		OsArgs:        []string{"dpm"},
		CmdPreRunHook: hook,
	}
}

// waitForFile blocks until path exists and is non-empty, so a test never
// signals a component that has not installed its handler yet.
func waitForFile(t *testing.T, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never appeared", filepath.Base(path))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A component that handles SIGTERM must be given the chance to. SIGKILL cannot
// be trapped, so the marker file only exists if the component was asked to stop
// rather than killed outright.
func TestExecSdkCommandSignalsComponentOnCancel(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	marker := filepath.Join(dir, "terminated")

	script := "trap 'printf caught > " + marker + "; exit 0' TERM; " +
		"printf ready > " + ready + "; " +
		"while true; do sleep 0.05; done"

	ctx, cancel := context.WithCancel(context.Background())
	da := newTestAssistant(t, nil)

	done := make(chan error, 1)
	go func() {
		_, err := da.execSdkCommand(ctx, "/bin/sh", []string{"-c", script}, nil)
		done <- err
	}()

	waitForFile(t, ready, 5*time.Second)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("component did not exit after the context was cancelled")
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatal("component was killed without being asked to stop: " +
			"its SIGTERM handler never ran")
	}
}

// A component that ignores SIGTERM must still be stopped, and only after the
// grace period. Both halves matter: dying instantly would mean the component
// was never asked, and never dying would mean no escalation policy at all.
func TestExecSdkCommandEscalatesWhenComponentIgnoresSignal(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	const grace = 300 * time.Millisecond

	script := "trap '' TERM; printf ready > " + ready + "; " +
		"while true; do sleep 0.05; done"

	ctx, cancel := context.WithCancel(context.Background())
	da := newTestAssistant(t, func(cmd *exec.Cmd) {
		cmd.WaitDelay = grace
	})

	done := make(chan struct{})
	go func() {
		_, _ = da.execSdkCommand(ctx, "/bin/sh", []string{"-c", script}, nil)
		close(done)
	}()

	waitForFile(t, ready, 5*time.Second)
	start := time.Now()
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a component ignoring SIGTERM was never stopped: " +
			"there is no escalation policy")
	}

	if elapsed := time.Since(start); elapsed < grace {
		t.Fatalf("component stopped after %v, before the %v grace period: "+
			"it was killed rather than asked", elapsed, grace)
	}
}
