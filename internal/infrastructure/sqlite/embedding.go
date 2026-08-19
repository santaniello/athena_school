package sqlite

import (
	"encoding/binary"
	"fmt"
	"math"
)

// bytesPerFloat32 is the width of one packed component: 4 bytes, no header.
const bytesPerFloat32 = 4

// encodeEmbedding packs vec as tightly-packed little-endian IEEE-754
// binary32: 4 bytes per component, no header. Dimension is len(blob)/4.
func encodeEmbedding(vec []float32) []byte {
	blob := make([]byte, len(vec)*bytesPerFloat32)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(blob[i*bytesPerFloat32:], math.Float32bits(v))
	}
	return blob
}

// decodeEmbedding unpacks blob into its float32 components. It returns an
// error when len(blob) is not a multiple of 4, since a component cannot be
// read from a partial 4-byte group.
func decodeEmbedding(blob []byte) ([]float32, error) {
	if len(blob)%bytesPerFloat32 != 0 {
		return nil, fmt.Errorf("sqlite: decoding embedding: blob length %d is not a multiple of %d", len(blob), bytesPerFloat32)
	}
	vec := make([]float32, len(blob)/bytesPerFloat32)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*bytesPerFloat32:]))
	}
	return vec, nil
}
