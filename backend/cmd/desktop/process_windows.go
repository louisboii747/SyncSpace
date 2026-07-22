//go:build windows

package main

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureBackendProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

type jobHandle struct{ handle windows.Handle }

func (j *jobHandle) Close() error { return windows.CloseHandle(j.handle) }

func attachBackendProcess(process *os.Process) (io.Closer, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	owner := &jobHandle{handle: job}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		owner.Close()
		return nil, err
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		owner.Close()
		return nil, err
	}
	defer windows.CloseHandle(processHandle)
	if err = windows.AssignProcessToJobObject(job, processHandle); err != nil {
		owner.Close()
		return nil, err
	}
	return owner, nil
}

func interruptBackend(process *os.Process) error { return process.Kill() }
