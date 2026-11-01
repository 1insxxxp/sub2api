package geminitokens

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLocalGeminiTokens_PythonEquivalence(t *testing.T) {
	raw, err := os.ReadFile("testdata/python_counts.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string   `json:"name"`
		Texts []string `json:"texts"`
		Raw   int      `json:"raw"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := CountTexts(tc.Texts)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.Raw {
				t.Fatalf("raw tokens=%d, Python SentencePiece=%d", got, tc.Raw)
			}
		})
	}
}

func TestLocalGeminiTokens_BoundsAndEmpty(t *testing.T) {
	if got, err := CountTexts([]string{"", ""}); err != nil || got != 0 {
		t.Fatalf("empty=%d,%v", got, err)
	}
	if _, err := CountTexts([]string{strings.Repeat("a", 4<<20), "b"}); err == nil {
		t.Fatal("oversized input accepted")
	}
	if _, err := CountTexts([]string{string([]byte{0xff})}); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestLocalGeminiTokens_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := CountTexts([]string{"请仅回复OK，不要解释。"})
			if err != nil || got != 8 {
				t.Errorf("count=%d err=%v", got, err)
			}
		}()
	}
	wg.Wait()
}
