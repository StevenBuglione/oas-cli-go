//go:build !linux

package main

import "os/exec"

func configureManagedRuntimePlatform(cmd *exec.Cmd) {}
