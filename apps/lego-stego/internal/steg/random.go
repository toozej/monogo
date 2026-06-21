package steg

import (
	"crypto/sha256"
	"encoding/binary"
)

type deterministicPRNG struct {
	key      [32]byte
	counter  uint64
	block    [32]byte
	blockPos int
}

func seededPRNG(password string) *deterministicPRNG {
	return &deterministicPRNG{
		key:      sha256.Sum256([]byte(password)),
		blockPos: 32,
	}
}

func (p *deterministicPRNG) refill() {
	var input [40]byte
	copy(input[:32], p.key[:])
	binary.BigEndian.PutUint64(input[32:], p.counter)
	p.block = sha256.Sum256(input[:])
	p.counter++
	p.blockPos = 0
}

func (p *deterministicPRNG) nextUint64() uint64 {
	var bytes [8]byte
	for i := range bytes {
		if p.blockPos >= len(p.block) {
			p.refill()
		}
		bytes[i] = p.block[p.blockPos]
		p.blockPos++
	}

	return binary.BigEndian.Uint64(bytes[:])
}

func (p *deterministicPRNG) Intn(n int) int {
	if n <= 0 {
		panic("invalid argument to Intn")
	}
	if n == 1 {
		return 0
	}

	bound := uint64(n)
	limit := ^uint64(0) - (^uint64(0) % bound)

	for {
		v := p.nextUint64()
		if v < limit {
			// #nosec G115 -- v % bound is guaranteed < bound, and bound comes from positive int n.
			return int(v % bound)
		}
	}
}

func (p *deterministicPRNG) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		j := p.Intn(i + 1)
		swap(i, j)
	}
}
