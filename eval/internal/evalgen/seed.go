package evalgen

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// stream is a splitmix64 generator seeded from the generator version, the cell and the seed, so the same
// inputs always draw the same values on every platform.
type stream struct{ state uint64 }

func newStream(version string, cell Cell, seed int64) *stream {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%+v|%d", version, cell, seed)))
	return &stream{state: binary.BigEndian.Uint64(sum[:8])}
}

func (s *stream) next() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

func (s *stream) intn(n int) int { return int(s.next() % uint64(n)) }

func pick[T any](s *stream, items []T) T { return items[s.intn(len(items))] }

func shuffled[T any](s *stream, items []T) []T {
	out := append([]T(nil), items...)
	for i := len(out) - 1; i > 0; i-- {
		j := s.intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}
