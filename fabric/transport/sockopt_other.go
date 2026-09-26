//go:build !linux

package transport

import "net"

// setUserTimeout is Linux-only (TCP_USER_TIMEOUT); elsewhere the default
// retransmission behaviour applies. The orderers always run on Linux.
func setUserTimeout(conn net.Conn, ms int) {}
