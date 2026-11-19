package desync

import (
	"encoding/binary"
	"fmt"
)

const tlsRecordHeaderLen = 5

// splitTLSRecord re-frames a single TLS record into several records, cutting
// the payload just after the first byte of the SNI host name and again in
// the middle of it. Each record is sent in its own TCP segment.
//
// RFC 8446 section 5.1 allows a handshake message to be fragmented across
// records, so servers reassemble it transparently, while DPI middleboxes
// that only parse the first record never see the complete host name. This
// works even when the DPI reassembles TCP segments, unlike plain splitting.
func splitTLSRecord(
	raw []byte,
	sniStart, sniEnd int,
	disorder bool,
) ([]Segment, error) {
	if len(raw) <= tlsRecordHeaderLen {
		return nil, fmt.Errorf("record too short")
	}
	if sniStart <= tlsRecordHeaderLen || sniEnd <= sniStart || sniEnd > len(raw) {
		return nil, fmt.Errorf("invalid sni range %d-%d", sniStart, sniEnd)
	}
	payloadLen := int(binary.BigEndian.Uint16(raw[3:5]))
	if payloadLen != len(raw)-tlsRecordHeaderLen {
		return nil, fmt.Errorf("multiple or truncated records are not supported")
	}

	mid := sniStart + max(1, (sniEnd-sniStart)/2)
	cuts := []int{sniStart + 1, mid, len(raw)}

	segments := make([]Segment, 0, len(cuts))
	prev := tlsRecordHeaderLen
	lazy := true
	for _, c := range cuts {
		if c <= prev {
			continue
		}
		part := raw[prev:c]
		rec := make([]byte, 0, tlsRecordHeaderLen+len(part))
		rec = append(rec, raw[:3]...) // content type + legacy version
		rec = binary.BigEndian.AppendUint16(rec, uint16(len(part)))
		rec = append(rec, part...)
		segments = append(segments, Segment{Packet: rec, Lazy: lazy && disorder})
		lazy = !lazy
		prev = c
	}

	return segments, nil
}
