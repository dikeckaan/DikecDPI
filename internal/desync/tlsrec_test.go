package desync

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/proto"
)

// realClientHello captures a ClientHello produced by crypto/tls.
func realClientHello(t *testing.T, sni string) []byte {
	t.Helper()
	c, s := net.Pipe()
	defer func() { _ = s.Close() }()
	go func() {
		cl := tls.Client(c, &tls.Config{ServerName: sni, InsecureSkipVerify: true})
		_ = cl.SetDeadline(time.Now().Add(2 * time.Second))
		_ = cl.Handshake()
		_ = cl.Close()
	}()
	_ = s.SetDeadline(time.Now().Add(2 * time.Second))
	hdr := make([]byte, 5)
	_, err := io.ReadFull(s, hdr)
	require.NoError(t, err)
	body := make([]byte, binary.BigEndian.Uint16(hdr[3:5]))
	_, err = io.ReadFull(s, body)
	require.NoError(t, err)
	return append(hdr, body...)
}

func TestSplitTLSRecord(t *testing.T) {
	raw := realClientHello(t, "discord.com")
	msg := proto.NewFakeTLSMessage(raw)
	start, end, err := msg.ExtractSNIOffset()
	require.NoError(t, err)
	sni := string(raw[start:end])
	require.Equal(t, "discord.com", sni)

	segs, err := splitTLSRecord(raw, start, end, false)
	require.NoError(t, err)
	assert.Len(t, segs, 3)

	var payload []byte
	for _, s := range segs {
		p := s.Packet
		assert.Equal(t, raw[:3], p[:3], "record header must be kept")
		assert.Equal(t, len(p)-5, int(binary.BigEndian.Uint16(p[3:5])))
		assert.False(
			t,
			strings.Contains(string(p), sni),
			"a record still holds the full SNI",
		)
		assert.False(t, s.Lazy)
		payload = append(payload, p[5:]...)
	}
	assert.True(t, bytes.Equal(payload, raw[5:]), "payload must be unchanged")
}

func TestSplitTLSRecordDisorder(t *testing.T) {
	raw := realClientHello(t, "example.org")
	msg := proto.NewFakeTLSMessage(raw)
	start, end, _ := msg.ExtractSNIOffset()
	segs, err := splitTLSRecord(raw, start, end, true)
	require.NoError(t, err)
	assert.True(t, segs[0].Lazy)
	assert.False(t, segs[1].Lazy)
}

func TestSplitTLSRecordRejectsBadInput(t *testing.T) {
	raw := []byte(config.FakeClientHello)
	_, err := splitTLSRecord(raw[:4], 0, 0, false)
	assert.Error(t, err)
	_, err = splitTLSRecord(raw, 10, 5, false)
	assert.Error(t, err)
	real := realClientHello(t, "example.org")
	_, err = splitTLSRecord(real[:len(real)-1], 50, 60, false)
	assert.Error(t, err, "truncated record must be rejected")
}

func TestSplitModeTLSRecordFallsBack(t *testing.T) {
	msg := proto.NewFakeTLSMessage([]byte{0x17, 0x03, 0x03, 0x00, 0x01, 0x00})
	opts := &config.HTTPSOptions{SplitMode: config.HTTPSSplitModeTLSRecord}
	segs := split(zerolog.Nop(), msg, opts)
	assert.Len(t, segs, 1)
}
