// Package geminitokens provides an offline text-only SentencePiece count using
// a pinned Google Gemma3 vocabulary. It is not a native Argon usage measurement.
package geminitokens

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"sync"
	"unicode/utf8"

	sentencepiece "github.com/eliben/go-sentencepiece"
)

const MaxTextBytes = 4 << 20

// ModelSHA256 is the checksum published by Google's GenAI tokenizer loader.
const ModelSHA256 = "1299c11d7cf632ef3b4e11937501358ada021bbdf7c47638d13c0ee982f2e79c"

//go:embed gemma3.model
var vocabulary []byte

var (
	once           sync.Once
	processor      *sentencepiece.Processor
	loadErr        error
	ErrUnavailable = errors.New("local Gemini input count unavailable")
)

// CountTexts sums independent text parts, matching the Google text tokenizer's
// accumulation semantics. No files, URLs or remote APIs are read at runtime.
func CountTexts(texts []string) (int, error) {
	bytesTotal := 0
	for _, text := range texts {
		if !utf8.ValidString(text) || len(text) > MaxTextBytes-bytesTotal {
			return 0, ErrUnavailable
		}
		bytesTotal += len(text)
	}
	once.Do(func() {
		sum := sha256.Sum256(vocabulary)
		if hex.EncodeToString(sum[:]) != ModelSHA256 {
			loadErr = ErrUnavailable
			return
		}
		processor, loadErr = sentencepiece.NewProcessor(bytes.NewReader(vocabulary))
	})
	if loadErr != nil || processor == nil {
		return 0, ErrUnavailable
	}
	total := 0
	for _, text := range texts {
		// The dependency expects a nonempty string when matching its first symbol.
		if text != "" {
			total += len(processor.Encode(text))
		}
	}
	return total, nil
}
