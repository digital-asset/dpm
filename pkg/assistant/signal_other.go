//go:build !unix

// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistant

import "os"

// terminateProcess stops a component. Windows has no SIGTERM that a process can
// handle, and os.Process.Signal rejects anything but os.Kill there, so this is
// the same abrupt stop exec.CommandContext already performed. Behaviour on
// Windows is unchanged by this file existing.
func terminateProcess(p *os.Process) error {
	return p.Kill()
}
