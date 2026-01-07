// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"errors"
	"github.com/juju/fslock"
	"log/slog"
	"path/filepath"
	"time"
)

// WithInstallLock performs an action guarded by a lockfile.
// This function blocks until the lock has been obtained.
// If the lock cannot be obtained immediately without blocking,
// a message is output.
//
// The lock is released after the action is performed, and is
// automatically released if the process ends prematurely.
func WithInstallLock(ctx context.Context, lockFilePath string, action func() error) error {
	if err := EnsureDirs(filepath.Dir(lockFilePath)); err != nil {
		return err
	}

	lock := fslock.New(lockFilePath)
	if err := lock.TryLock(); errors.Is(err, fslock.ErrLocked) {
		slog.Info("Waiting for installation lock... " + lockFilePath)
		if err := waitForLock(ctx, lock); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			slog.Warn("failure while releasing installation lock", "file", lockFilePath, "err", err.Error())
		}
	}()

	return func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return action()
		}
	}()
}

// waitForLock is needed because fslock doesn't provide a method that works with contexts
func waitForLock(ctx context.Context, lock *fslock.Lock) error {
	for {
		if err := lock.TryLock(); err == nil {
			return nil
		} else if !errors.Is(err, fslock.ErrLocked) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
