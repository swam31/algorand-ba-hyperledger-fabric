package transport

import (
	"net"
	"syscall"
)

// setUserTimeout makes the kernel drop a connection whose sent data has gone
// unacknowledged for ms milliseconds. Without it, a link that loses every
// packet for a while backs off its retransmissions exponentially and can stay
// silent for a minute or more after the network heals. Heartbeats keep data in
// flight, so a dead link is noticed within ms and redialled.
func setUserTimeout(conn net.Conn, ms int) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_USER_TIMEOUT, ms)
	})
}
