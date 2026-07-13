package engine

import (
	"syscall"
	"unsafe"
)

var (
	modIphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	procGetTcp   = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetUdp   = modIphlpapi.NewProc("GetExtendedUdpTable")

	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	afInet6        = 23
	tcpTableOwnerPidAll = 5
	udpTableOwnerPid    = 1
)

type mibTCPRowOwnerPID struct {
	dwState      uint32
	dwLocalAddr  uint32
	dwLocalPort  uint32
	dwRemoteAddr uint32
	dwRemotePort uint32
	dwOwningPid  uint32
}

type mibTCPTableOwnerPID struct {
	dwNumEntries uint32
	table        [1]mibTCPRowOwnerPID
}

type mibUDPRowOwnerPID struct {
	dwLocalAddr uint32
	dwLocalPort uint32
	dwOwningPid uint32
}

type mibUDPTableOwnerPID struct {
	dwNumEntries uint32
	table        [1]mibUDPRowOwnerPID
}

func collectActiveProcs(out map[string]int) {
	collectTcpTable(out, afInet6)
	collectTcpTable(out, 2) // AF_INET
	collectUdpTable(out, afInet6)
	collectUdpTable(out, 2)
}

func collectTcpTable(out map[string]int, af uint32) {
	size := uint32(8 * 1024)
	buf := make([]byte, size)
	r, _, _ := procGetTcp.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		1,
		uintptr(af),
		uintptr(tcpTableOwnerPidAll),
		0,
	)
	if r != 0 {
		if r == 122 /*ERROR_INSUFFICIENT_BUFFER*/ {
			buf = make([]byte, size)
			r, _, _ = procGetTcp.Call(
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(unsafe.Pointer(&size)),
				1,
				uintptr(af),
				uintptr(tcpTableOwnerPidAll),
				0,
			)
		}
		if r != 0 {
			return
		}
	}
	hdr := (*mibTCPTableOwnerPID)(unsafe.Pointer(&buf[0]))
	n := int(hdr.dwNumEntries)
	rows := unsafe.Slice(&hdr.table[0], n)
	for i := 0; i < n; i++ {
		addPid(out, rows[i].dwOwningPid)
	}
}

func collectUdpTable(out map[string]int, af uint32) {
	size := uint32(8 * 1024)
	buf := make([]byte, size)
	r, _, _ := procGetUdp.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		1,
		uintptr(af),
		uintptr(udpTableOwnerPid),
		0,
	)
	if r != 0 {
		if r == 122 {
			buf = make([]byte, size)
			r, _, _ = procGetUdp.Call(
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(unsafe.Pointer(&size)),
				1,
				uintptr(af),
				uintptr(udpTableOwnerPid),
				0,
			)
		}
		if r != 0 {
			return
		}
	}
	hdr := (*mibUDPTableOwnerPID)(unsafe.Pointer(&buf[0]))
	n := int(hdr.dwNumEntries)
	rows := unsafe.Slice(&hdr.table[0], n)
	for i := 0; i < n; i++ {
		addPid(out, rows[i].dwOwningPid)
	}
}

func addPid(out map[string]int, pid uint32) {
	if pid == 0 || pid == 4 { // System Idle / System
		return
	}
	name := processName(pid)
	if name == "" {
		return
	}
	out[name]++
}

func processName(pid uint32) string {
	const procQueryInfo = 0x0400
	h, err := syscall.OpenProcess(procQueryInfo, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, 260)
	length := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(h),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&length)),
	)
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf[:length])
	// base name after last backslash
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
