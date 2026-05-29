package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tealife/proxy-cs3/src/tunnel"
)

// TestCovFragment_NoStream tests handleFragmentPacket with no existing stream
func TestCovFragment_NoStream(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	fragPkt := tunnel.NewFragmentPacket(
		"conn-frag-nost",
		"s-no-exist",
		tunnel.PacketTypeData,
		1, 2, 0, 0,
		[]byte("frag-data"),
	)
	err := sc.ProcessIncomingData(fragPkt.Bytes())
	assert.NoError(t, err)
}

// TestCovFragment_WithStream tests handleFragmentPacket with existing stream
func TestCovFragment_WithStream(t *testing.T) {
	sc, conn := newTestSC(t)
	defer conn.Close()
	sc.Start()
	defer sc.Close()

	stream := newServerStream("s-frag", sc)
	sc.AddStream("s-frag", stream)

	fragPkt := tunnel.NewFragmentPacket(
		"conn-frag-exist",
		"s-frag",
		tunnel.PacketTypeData,
		1, 2, 0, 0,
		[]byte("frag-data-1"),
	)
	err := sc.ProcessIncomingData(fragPkt.Bytes())
	require.NoError(t, err)

	fragPkt2 := tunnel.NewFragmentPacket(
		"conn-frag-exist",
		"s-frag",
		tunnel.PacketTypeData,
		2, 2, 1, 0,
		[]byte("frag-data-2"),
	)
	err = sc.ProcessIncomingData(fragPkt2.Bytes())
	assert.NoError(t, err)
}
