package ecc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"

	"github.com/klauspost/reedsolomon"
)

const (
	DataShards   = 10
	ParityShards = 3
	TotalShards  = DataShards + ParityShards
	OrigLenSize  = 4
)

var (
	ErrInvalidData = errors.New("ecc: invalid data")
	ErrVerifyFail  = errors.New("ecc: verification failed")
)

func Encode(data []byte) ([]byte, error) {
	enc, err := reedsolomon.New(DataShards, ParityShards)
	if err != nil {
		return nil, err
	}

	shards, err := enc.Split(data)
	if err != nil {
		return nil, err
	}

	if err := enc.Encode(shards); err != nil {
		return nil, err
	}

	if uint64(len(data)) > math.MaxUint32 {
		return nil, ErrInvalidData
	}
	lenBuf := make([]byte, OrigLenSize)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data))) // #nosec G115

	var buf bytes.Buffer
	buf.Write(lenBuf)
	for _, s := range shards {
		buf.Write(s)
	}

	return buf.Bytes(), nil
}

func Decode(data []byte) ([]byte, error) {
	return DecodeWithMissing(data, nil)
}

func DecodeWithMissing(data []byte, missing []int) ([]byte, error) {
	if len(data) < OrigLenSize {
		return nil, ErrInvalidData
	}

	origLen := binary.BigEndian.Uint32(data[:OrigLenSize])
	data = data[OrigLenSize:]

	enc, err := reedsolomon.New(DataShards, ParityShards)
	if err != nil {
		return nil, err
	}

	if len(data)%TotalShards != 0 {
		return nil, ErrInvalidData
	}

	shardSize := len(data) / TotalShards

	shards := make([][]byte, TotalShards)
	for i := range shards {
		start := i * shardSize
		end := start + shardSize
		shards[i] = data[start:end]
	}

	missingSet := make(map[int]bool)
	for _, m := range missing {
		missingSet[m] = true
	}

	for i := range missing {
		if missing[i] >= 0 && missing[i] < TotalShards {
			shards[missing[i]] = nil
		}
	}

	if err := enc.Reconstruct(shards); err != nil {
		return nil, err
	}

	if ok, err := enc.Verify(shards); !ok || err != nil {
		return nil, ErrVerifyFail
	}

	var buf bytes.Buffer
	for i := 0; i < DataShards; i++ {
		buf.Write(shards[i])
	}

	result := buf.Bytes()
	if len(result) >= int(origLen) { // #nosec G115
		result = result[:origLen]
	}

	return result, nil
}
