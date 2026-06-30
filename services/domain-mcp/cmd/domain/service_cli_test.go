package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const procNetTCPFixture = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F40 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 123456 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1F41 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 654321 1 0000000000000000 100 0 0 10 0
   2: 0100007F:1F40 0100007F:9C40 01 00000000:00000000 00:00000000 00000000  1000        0 111111 1 0000000000000000 100 0 0 10 0
`

func TestListenInodesForPort_MatchesListenOnly(t *testing.T) {

	inodes := listenInodesForPort(procNetTCPFixture, 8000)
	require.Equal(t, []string{"123456"}, inodes)

	inodes = listenInodesForPort(procNetTCPFixture, 8001)
	require.Equal(t, []string{"654321"}, inodes)

	require.Empty(t, listenInodesForPort(procNetTCPFixture, 9999))
}

func TestPortFromBaseURL(t *testing.T) {
	require.Equal(t, 8042, portFromBaseURL("http://localhost:8042"))
	require.Equal(t, 8000, portFromBaseURL("http://localhost"))
	require.Equal(t, 8000, portFromBaseURL("::bad::"))
}

func TestServiceUnitContent_Shape(t *testing.T) {
	unit := serviceUnitContent("/home/x/go/bin/domain")
	require.Contains(t, unit, "ExecStart=/home/x/go/bin/domain server")
	require.Contains(t, unit, "Restart=always")
	require.Contains(t, unit, "WantedBy=default.target")
	require.False(t, strings.Contains(unit, "EnvironmentFile"),
		"sin EnvironmentFile: el binario carga config en cascada solo")
}
