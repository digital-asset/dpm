//go:build unix

// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"os"
	"syscall"
)

// terminateProcess asks a component to stop. SIGTERM can be handled, so a
// component that wants to flush state or remove a lock file gets the chance.
// Escalation to SIGKILL is left to exec.Cmd.WaitDelay rather than done here.
func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
