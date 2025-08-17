//go:build !wasm && nostylus

package arbcompress

import "errors"

type brotliBool = uint32

func CompressWell(input []byte) ([]byte, error) {
	return Compress(input, LEVEL_WELL, EmptyDictionary)
}

func Compress(input []byte, level uint32, dictionary Dictionary) ([]byte, error) {
	out := make([]byte, len(input))
	copy(out, input)
	return out, nil
}

var ErrOutputWontFit = errors.New("output won't fit in maxsize")

func Decompress(input []byte, maxSize int) ([]byte, error) {
	return DecompressWithDictionary(input, maxSize, EmptyDictionary)
}

func DecompressWithDictionary(input []byte, maxSize int, dictionary Dictionary) ([]byte, error) {
	if len(input) > maxSize {
		return nil, ErrOutputWontFit
	}
	out := make([]byte, len(input))
	copy(out, input)
	return out, nil
}
