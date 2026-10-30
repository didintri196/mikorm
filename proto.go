package mikorm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// sentence is one reply sentence read from the device.
type sentence struct {
	word  string // !re, !done, !trap, !fatal, !empty
	tag   string
	attrs map[string]string
}

// writeSentence encodes words, followed by an optional tag word and the
// terminating empty word, to w.
func writeSentence(w *bufio.Writer, words []string, tag string) error {
	for _, word := range words {
		writeWord(w, word)
	}
	if tag != "" {
		writeWord(w, ".tag="+tag)
	}
	writeWord(w, "")
	return w.Flush()
}

func writeWord(w *bufio.Writer, word string) {
	n := uint32(len(word))
	switch {
	case n < 0x80:
		w.WriteByte(byte(n))
	case n < 0x4000:
		w.Write([]byte{byte(n>>8) | 0x80, byte(n)})
	case n < 0x200000:
		w.Write([]byte{byte(n>>16) | 0xC0, byte(n >> 8), byte(n)})
	case n < 0x10000000:
		w.Write([]byte{byte(n>>24) | 0xE0, byte(n >> 16), byte(n >> 8), byte(n)})
	default:
		w.Write([]byte{0xF0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	w.WriteString(word)
}

// readWords reads the raw words of one sentence.
func readWords(r *bufio.Reader) ([]string, error) {
	var words []string
	for {
		n, err := readLength(r)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return words, nil
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		words = append(words, string(b))
	}
}

func readLength(r *bufio.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	var extra int
	var n uint32
	switch {
	case b&0x80 == 0x00:
		return int(b), nil
	case b&0xC0 == 0x80:
		extra, n = 1, uint32(b&0x3F)
	case b&0xE0 == 0xC0:
		extra, n = 2, uint32(b&0x1F)
	case b&0xF0 == 0xE0:
		extra, n = 3, uint32(b&0x0F)
	case b == 0xF0:
		var buf [4]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint32(buf[:])), nil
	default:
		return 0, fmt.Errorf("invalid word length prefix %#x", b)
	}
	for i := 0; i < extra; i++ {
		c, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n = n<<8 | uint32(c)
	}
	return int(n), nil
}

// readSentence reads and parses one reply sentence.
func readSentence(r *bufio.Reader) (*sentence, error) {
	for {
		words, err := readWords(r)
		if err != nil {
			return nil, err
		}
		if len(words) == 0 {
			continue // empty sentences must be ignored
		}
		s := &sentence{word: words[0], attrs: make(map[string]string, len(words)-1)}
		for _, w := range words[1:] {
			switch {
			case strings.HasPrefix(w, ".tag="):
				s.tag = w[len(".tag="):]
			case strings.HasPrefix(w, "="):
				k, v, _ := strings.Cut(w[1:], "=")
				s.attrs[k] = v
			default:
				// !fatal carries its message as a bare word.
				if s.attrs["message"] == "" {
					s.attrs["message"] = w
				}
			}
		}
		return s, nil
	}
}
