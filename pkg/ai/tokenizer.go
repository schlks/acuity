package ai

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
)

//go:embed bpe_simple_vocab_16e6.txt.gz
var bpeVocabGz []byte

type pair struct {
	first, second string
}

type CLIPTokenizer struct {
	byteEncoder map[byte]string
	byteDecoder map[string]byte
	encoder     map[string]int
	decoder     map[int]string
	bpeRanks    map[pair]int
	cache       map[string]string
	cacheMu     sync.RWMutex
	pat         *regexp.Regexp
}

func bytesToUnicode() (map[byte]string, map[string]byte) {
	byteEncoder := make(map[byte]string)
	byteDecoder := make(map[string]byte)

	var bs []int
	for b := int('!'); b <= int('~'); b++ {
		bs = append(bs, b)
	}
	for b := int('¡'); b <= int('¬'); b++ {
		bs = append(bs, b)
	}
	for b := int('®'); b <= int('ÿ'); b++ {
		bs = append(bs, b)
	}

	cs := make([]int, len(bs))
	copy(cs, bs)

	n := 0
	for b := 0; b < 256; b++ {
		found := false
		for _, v := range bs {
			if v == b {
				found = true
				break
			}
		}
		if !found {
			bs = append(bs, b)
			cs = append(cs, 256+n)
			n++
		}
	}

	for i := range bs {
		b := byte(bs[i])
		s := string(rune(cs[i]))
		byteEncoder[b] = s
		byteDecoder[s] = b
	}

	return byteEncoder, byteDecoder
}

func NewCLIPTokenizer() (*CLIPTokenizer, error) {
	byteEncoder, byteDecoder := bytesToUnicode()

	gz, err := gzip.NewReader(bytes.NewReader(bpeVocabGz))
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded vocab gz: %w", err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	var merges []pair
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 {
			merges = append(merges, pair{first: parts[0], second: parts[1]})
		}
	}

	bpeRanks := make(map[pair]int, len(merges))
	if len(merges) > 48894 {
		merges = merges[:48894]
	}
	for i, m := range merges {
		bpeRanks[m] = i
	}

	vocab := make([]string, 0, 49408)
	var byteList []string
	for b := 0; b < 256; b++ {
		byteList = append(byteList, byteEncoder[byte(b)])
	}
	for _, v := range byteList {
		vocab = append(vocab, v)
	}
	for _, v := range byteList {
		vocab = append(vocab, v+"</w>")
	}
	for _, m := range merges {
		vocab = append(vocab, m.first+m.second)
	}
	vocab = append(vocab, "<|startoftext|>", "<|endoftext|>")

	encoder := make(map[string]int, len(vocab))
	decoder := make(map[int]string, len(vocab))
	for i, v := range vocab {
		encoder[v] = i
		decoder[i] = v
	}

	pat := regexp.MustCompile(`(?i)<\|startoftext\|>|<\|endoftext\|>|'s|'t|'re|'ve|'m|'ll|'d|\p{L}+|\p{N}+|[^\s\p{L}\p{N}]+`)

	return &CLIPTokenizer{
		byteEncoder: byteEncoder,
		byteDecoder: byteDecoder,
		encoder:     encoder,
		decoder:     decoder,
		bpeRanks:    bpeRanks,
		cache: map[string]string{
			"<|startoftext|>": "<|startoftext|>",
			"<|endoftext|>":   "<|endoftext|>",
		},
		pat: pat,
	}, nil
}

func getPairs(word []string) []pair {
	if len(word) < 2 {
		return nil
	}
	pairs := make([]pair, len(word)-1)
	for i := 0; i < len(word)-1; i++ {
		pairs[i] = pair{first: word[i], second: word[i+1]}
	}
	return pairs
}

func (t *CLIPTokenizer) bpe(token string) string {
	t.cacheMu.RLock()
	if val, ok := t.cache[token]; ok {
		t.cacheMu.RUnlock()
		return val
	}
	t.cacheMu.RUnlock()

	word := make([]string, 0, len(token))
	runes := []rune(token)
	for i := 0; i < len(runes)-1; i++ {
		word = append(word, string(runes[i]))
	}
	word = append(word, string(runes[len(runes)-1])+"</w>")

	for len(word) > 1 {
		pairs := getPairs(word)
		minRank := math.MaxInt32
		var bestPair pair
		found := false

		for _, p := range pairs {
			if rank, ok := t.bpeRanks[p]; ok {
				if rank < minRank {
					minRank = rank
					bestPair = p
					found = true
				}
			}
		}

		if !found {
			break
		}

		var newWord []string
		i := 0
		for i < len(word) {
			if i < len(word)-1 && word[i] == bestPair.first && word[i+1] == bestPair.second {
				newWord = append(newWord, bestPair.first+bestPair.second)
				i += 2
			} else {
				newWord = append(newWord, word[i])
				i++
			}
		}
		word = newWord
	}

	result := strings.Join(word, " ")
	t.cacheMu.Lock()
	t.cache[token] = result
	t.cacheMu.Unlock()
	return result
}

func (t *CLIPTokenizer) Encode(text string) ([]int, error) {
	cleanText := strings.ToLower(strings.TrimSpace(text))
	matches := t.pat.FindAllString(cleanText, -1)

	bpeTokens := []int{t.encoder["<|startoftext|>"]}
	for _, token := range matches {
		var encodedToken strings.Builder
		for _, b := range []byte(token) {
			encodedToken.WriteString(t.byteEncoder[b])
		}
		bpeSubwords := strings.Fields(t.bpe(encodedToken.String()))
		for _, bpeSubword := range bpeSubwords {
			if id, ok := t.encoder[bpeSubword]; ok {
				bpeTokens = append(bpeTokens, id)
			}
		}
	}
	bpeTokens = append(bpeTokens, t.encoder["<|endoftext|>"])

	if len(bpeTokens) > 77 {
		bpeTokens = bpeTokens[:76]
		bpeTokens = append(bpeTokens, t.encoder["<|endoftext|>"])
	}

	return bpeTokens, nil
}
