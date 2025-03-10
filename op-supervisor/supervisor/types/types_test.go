package types

import (
	"encoding/binary"
	"encoding/json"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
)

func FuzzRoundtripIdentifierJSONMarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, origin []byte, blockNumber uint64, logIndex uint32, timestamp uint64, chainID []byte) {
		if len(chainID) > 32 {
			chainID = chainID[:32]
		}

		id := Identifier{
			Origin:      common.BytesToAddress(origin),
			BlockNumber: blockNumber,
			LogIndex:    logIndex,
			Timestamp:   timestamp,
			ChainID:     eth.ChainIDFromBig(new(big.Int).SetBytes(chainID)),
		}

		raw, err := json.Marshal(&id)
		require.NoError(t, err)

		var dec Identifier
		require.NoError(t, json.Unmarshal(raw, &dec))

		require.Equal(t, id.Origin, dec.Origin)
		require.Equal(t, id.BlockNumber, dec.BlockNumber)
		require.Equal(t, id.LogIndex, dec.LogIndex)
		require.Equal(t, id.Timestamp, dec.Timestamp)
		require.Equal(t, id.ChainID, dec.ChainID)
	})
}

func TestHashing(t *testing.T) {
	keccak256 := func(name string, parts ...[]byte) (h common.Hash) {
		t.Logf("%s = H(", name)
		for _, p := range parts {
			t.Logf("  %x,", p)
		}
		t.Logf(")")
		h = crypto.Keccak256Hash(parts...)
		t.Logf("%s = %s", name, h)
		return h
	}
	id := Identifier{
		Origin:      common.Address{},
		BlockNumber: 0xa1a2_a3a4_a5a6_a7a8,
		LogIndex:    0xb1b2_b3b4,
		Timestamp:   0xc1c2_c3c4_c5c6_c7c8,
		ChainID:     eth.ChainIDFromUInt64(0xd1d2_d3d4_d5d6_d7d8),
	}
	payloadHash := keccak256("payloadHash", []byte("example payload")) // aka msgHash
	logHash := keccak256("logHash", id.Origin[:], payloadHash[:])
	x := PayloadHashToLogHash(payloadHash, id.Origin)
	require.Equal(t, logHash, x, "check op-supervisor version of log-hashing matches intermediate value")

	var idPacked []byte
	idPacked = append(idPacked, make([]byte, 12)...)
	idPacked = binary.BigEndian.AppendUint64(idPacked, id.BlockNumber)
	idPacked = binary.BigEndian.AppendUint64(idPacked, id.Timestamp)
	idPacked = binary.BigEndian.AppendUint32(idPacked, id.LogIndex)
	t.Logf("idPacked: %x", idPacked)

	idLogHash := keccak256("idLogHash", logHash[:], idPacked)
	chainID := id.ChainID.Bytes32()
	bareChecksum := keccak256("bareChecksum", idLogHash[:], chainID[:])

	checksum := bareChecksum
	checksum[0] = 0x03
	t.Logf("Checksum: %s", checksum)
}
