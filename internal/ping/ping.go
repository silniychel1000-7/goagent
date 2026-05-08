// Package ping implements ICMP echo probing using only the Go standard library.
// It requires NET_RAW capability (or root) to open a raw socket.
package ping

import (
	"encoding/binary"
	"net"
	"os"
	"time"
)

// Result holds the outcome of a single ping check.
type Result struct {
	Target     string
	Up         bool
	LatencyMs  float64
	PacketLoss float64
	Error      error
}

// Pinger checks network reachability via ICMP echo request/reply.
type Pinger struct {
	timeout time.Duration
	count   int
}

// New creates a Pinger with the given timeout per round-trip.
func New(timeout time.Duration) *Pinger {
	return &Pinger{
		timeout: timeout,
		count:   3,
	}
}

// Ping sends `count` ICMP echo requests to target and returns a Result.
func (p *Pinger) Ping(target string) Result {
	r := Result{Target: target}

	ip, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		r.Error = err
		r.PacketLoss = 100
		return r
	}

	conn, err := net.DialTimeout("ip4:icmp", ip.String(), p.timeout)
	if err != nil {
		// Fallback: try TCP reachability check (no raw socket needed).
		return p.tcpProbe(target)
	}
	defer conn.Close()

	id := uint16(os.Getpid() & 0xffff)
	sent := 0
	received := 0
	var totalRTT time.Duration

	for seq := 0; seq < p.count; seq++ {
		msg := makeICMPEcho(id, uint16(seq))

		start := time.Now()
		_ = conn.SetDeadline(time.Now().Add(p.timeout))

		if _, err := conn.Write(msg); err != nil {
			continue
		}
		sent++

		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil || n < 28 { // 20 IP hdr + 8 ICMP
			continue
		}

		rtt := time.Since(start)

		// Skip 20-byte IP header; byte 20 is ICMP type.
		if buf[20] == 0 { // type 0 = echo reply
			received++
			totalRTT += rtt
		}
	}

	if sent == 0 {
		r.PacketLoss = 100
		return r
	}

	r.PacketLoss = float64(sent-received) / float64(sent) * 100
	r.Up = received > 0
	if received > 0 {
		r.LatencyMs = float64(totalRTT/time.Duration(received)) / float64(time.Millisecond)
	}
	return r
}

// tcpProbe is a fallback reachability check via TCP connect.
func (p *Pinger) tcpProbe(target string) Result {
	r := Result{Target: target}
	for _, port := range []string{"80", "443", "53"} {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, port), p.timeout)
		if err == nil {
			conn.Close()
			r.Up = true
			r.LatencyMs = float64(time.Since(start)) / float64(time.Millisecond)
			return r
		}
	}
	r.PacketLoss = 100
	return r
}

func makeICMPEcho(id, seq uint16) []byte {
	msg := make([]byte, 8)
	msg[0] = 8 // echo request
	msg[1] = 0
	binary.BigEndian.PutUint16(msg[4:], id)
	binary.BigEndian.PutUint16(msg[6:], seq)
	binary.BigEndian.PutUint16(msg[2:], checksum(msg))
	return msg
}

func checksum(msg []byte) uint16 {
	sum := 0
	for i := 0; i+1 < len(msg); i += 2 {
		sum += int(msg[i])<<8 | int(msg[i+1])
	}
	if len(msg)%2 == 1 {
		sum += int(msg[len(msg)-1]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return uint16(^sum)
}
