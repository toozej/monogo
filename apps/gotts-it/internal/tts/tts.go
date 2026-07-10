// Package tts provides a thin client that sends text to an OpenAI-compatible
// text-to-speech endpoint (e.g. Speaches) and writes the returned audio
// bytes to a local file. Long inputs are automatically split at sentence
// boundaries (<= 4096 chars per chunk) and the resulting audio segments are
// concatenated.
package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"

	log "github.com/sirupsen/logrus"
)

// MaxInputChars mirrors the OpenAI TTS API's per-request input length cap.
const MaxInputChars = 4096

// Options holds the configuration for the TTS synthesis.
type Options struct {
	BaseURL      string
	APIKey       string
	Model        string
	Voice        string
	Format       string
	Speed        float64
	Instructions string
	Timeout      time.Duration
}

// Synthesize sends text to the configured TTS endpoint and streams the
// concatenated audio to outputPath. Long inputs are chunked at sentence
// boundaries. Returns an error on non-2xx, network failure, or file write
// failure.
func Synthesize(ctx context.Context, text, outputPath string, opts Options) error {
	chunks := chunk(text, MaxInputChars)
	if len(chunks) == 0 {
		return fmt.Errorf("no text to synthesize")
	}
	opts.Format = strings.ToLower(opts.Format)
	switch opts.Format {
	case "mp3", "wav", "flac", "pcm":
	default:
		return fmt.Errorf("unsupported audio format %q", opts.Format)
	}
	if opts.Format == "flac" && len(chunks) > 1 {
		return fmt.Errorf("FLAC chunked synthesis is not supported; use mp3 or wav")
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = "cant-be-empty"
	}

	client := openai.NewClient(
		option.WithBaseURL(opts.BaseURL),
		option.WithAPIKey(apiKey),
	)

	output, err := newAtomicOutput(outputPath)
	if err != nil {
		return err
	}
	defer output.abort()
	f := output.file

	format := responseFormat(opts.Format)

	start := time.Now()
	for i, chunkText := range chunks {
		log.WithField("chunk", fmt.Sprintf("%d/%d", i+1, len(chunks))).Debugf("synthesizing %d chars", len([]rune(chunkText)))

		chunkCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err := synthesizeChunk(chunkCtx, client, chunkText, opts, format)
		if err != nil {
			cancel()
			return fmt.Errorf("synthesize chunk %d/%d: %w", i+1, len(chunks), err)
		}

		if err := writeChunk(f, resp, opts.Format, i == 0, i == len(chunks)-1); err != nil {
			closeErr := resp.Body.Close()
			cancel()
			if closeErr != nil {
				return fmt.Errorf("write chunk %d/%d: %w (close response: %v)", i+1, len(chunks), err, closeErr)
			}
			return fmt.Errorf("write chunk %d/%d: %w", i+1, len(chunks), err)
		}
		if err := resp.Body.Close(); err != nil {
			cancel()
			return fmt.Errorf("close chunk %d/%d response: %w", i+1, len(chunks), err)
		}
		cancel()
	}

	if opts.Format == "wav" && len(chunks) > 1 {
		if err := rewriteWAVHeader(output.tempPath); err != nil {
			return fmt.Errorf("rewrite wav header: %w", err)
		}
	}
	if err := output.commit(); err != nil {
		return err
	}

	log.Infof("synthesized %d chunks in %s", len(chunks), time.Since(start).Round(time.Millisecond))
	return nil
}

func synthesizeChunk(ctx context.Context, client openai.Client, text string, opts Options, format openai.AudioSpeechNewParamsResponseFormat) (*http.Response, error) {
	params := openai.AudioSpeechNewParams{
		Input:          text,
		Model:          openai.SpeechModel(opts.Model),
		Voice:          openai.AudioSpeechNewParamsVoiceUnion{OfString: param.NewOpt(opts.Voice)},
		ResponseFormat: format,
	}
	if opts.Speed > 0 {
		params.Speed = param.NewOpt(opts.Speed)
	}
	if opts.Instructions != "" {
		params.Instructions = param.NewOpt(opts.Instructions)
	}

	resp, err := client.Audio.Speech.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		closeErr := resp.Body.Close()
		errMsg := fmt.Sprintf("tts request returned status %d: %s", resp.StatusCode, string(body))
		if closeErr != nil {
			return nil, fmt.Errorf("%s (additionally, close error: %w)", errMsg, closeErr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return resp, nil
}

func writeChunk(f *os.File, resp *http.Response, format string, isFirst, isLast bool) error {
	switch format {
	case "mp3", "pcm":
		_, err := io.Copy(f, resp.Body)
		return err
	case "wav":
		return writeWAVChunk(f, resp.Body, isFirst)
	case "flac":
		if !isFirst {
			return fmt.Errorf("FLAC chunked synthesis not yet supported; use mp3 or wav")
		}
		_, err := io.Copy(f, resp.Body)
		return err
	default:
		_, err := io.Copy(f, resp.Body)
		return err
	}
}

func writeWAVChunk(f *os.File, r io.Reader, isFirst bool) error {
	header, audio, err := parseWAVChunk(r)
	if err != nil {
		return err
	}
	if isFirst {
		if _, err := f.Write(header); err != nil {
			return err
		}
	}
	_, err = f.Write(audio)
	return err
}

func parseWAVChunk(r io.Reader) ([]byte, []byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	if len(data) < 12 || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return nil, nil, fmt.Errorf("invalid WAV response")
	}
	dataLen := int64(len(data))
	for offset := int64(12); offset+8 <= dataLen; {
		size := int64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		payloadStart := offset + 8
		payloadEnd := payloadStart + size
		if payloadEnd < payloadStart || payloadEnd > dataLen {
			return nil, nil, fmt.Errorf("invalid WAV chunk size")
		}
		if bytes.Equal(data[offset:offset+4], []byte("data")) {
			return data[:payloadStart], data[payloadStart:payloadEnd], nil
		}
		offset = payloadEnd + size%2
	}
	return nil, nil, fmt.Errorf("WAV response has no data chunk")
}

func responseFormat(format string) openai.AudioSpeechNewParamsResponseFormat {
	switch strings.ToLower(format) {
	case "wav":
		return openai.AudioSpeechNewParamsResponseFormatWAV
	case "flac":
		return openai.AudioSpeechNewParamsResponseFormatFLAC
	case "pcm":
		return openai.AudioSpeechNewParamsResponseFormatPCM
	default:
		return openai.AudioSpeechNewParamsResponseFormatMP3
	}
}

// chunk splits text into pieces of at most max characters, preferring
// sentence boundaries (. ! ? followed by whitespace, or double newline).
// If no boundary is found within max characters, a hard split is performed.
func chunk(text string, max int) []string {
	if max <= 0 || text == "" {
		return nil
	}
	remaining := []rune(text)
	if len(remaining) <= max {
		return []string{text}
	}

	var chunks []string
	for len(remaining) > max {
		prefix := string(remaining[:max])
		boundary := findSentenceBoundary(prefix, max)
		if boundary == 0 {
			chunks = append(chunks, prefix)
			remaining = remaining[max:]
			continue
		}
		split := len([]rune(prefix[:boundary]))
		chunks = append(chunks, string(remaining[:split]))
		remaining = remaining[split:]
	}
	if len(remaining) > 0 {
		chunks = append(chunks, string(remaining))
	}
	return chunks
}

func findSentenceBoundary(text string, max int) int {
	if max <= 0 {
		return 0
	}
	runes := []rune(text)
	if len(runes) > max {
		runes = runes[:max]
	}
	prefix := string(runes)

	separators := []string{". ", "! ", "? ", "\n\n"}
	best := 0
	for _, sep := range separators {
		idx := strings.LastIndex(prefix, sep)
		if idx > 0 {
			splitPoint := idx + len(sep)
			if splitPoint > best {
				best = splitPoint
			}
		}
	}
	return best
}

// rewriteWAVHeader rewrites the RIFF header of a WAV file at path to
// reflect the actual data chunk size after concatenation.
func rewriteWAVHeader(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", path, err)
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open root %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.OpenFile(base, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	fileSize := info.Size()
	if fileSize < 12 {
		return fmt.Errorf("wav file too small to rewrite header: %d bytes", fileSize)
	}
	header := make([]byte, 12)
	if _, err := f.ReadAt(header, 0); err != nil {
		return err
	}
	if !bytes.Equal(header[:4], []byte("RIFF")) || !bytes.Equal(header[8:12], []byte("WAVE")) {
		return fmt.Errorf("invalid WAV header")
	}

	dataSizeOffset := int64(-1)
	dataStart := int64(-1)
	for offset := int64(12); offset+8 <= fileSize; {
		chunkHeader := make([]byte, 8)
		if _, err := f.ReadAt(chunkHeader, offset); err != nil {
			return err
		}
		chunkSize := int64(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		if bytes.Equal(chunkHeader[:4], []byte("data")) {
			dataSizeOffset = offset + 4
			dataStart = offset + 8
			break
		}
		offset += 8 + chunkSize + chunkSize%2
	}
	if dataStart < 0 {
		return fmt.Errorf("WAV file has no data chunk")
	}

	riffSize := fileSize - 8
	dataSize := fileSize - dataStart

	if riffSize > math.MaxUint32 {
		return fmt.Errorf("riff size %d exceeds uint32 max", riffSize)
	}
	if dataSize > math.MaxUint32 {
		return fmt.Errorf("data size %d exceeds uint32 max", dataSize)
	}

	riffHeader := []byte("RIFF")
	if _, err := f.WriteAt(riffHeader, 0); err != nil {
		return err
	}
	if err := writeUint32LE(f, uint32(riffSize), 4); err != nil {
		return err
	}
	return writeUint32LE(f, uint32(dataSize), dataSizeOffset)
}

func writeUint32LE(f *os.File, val uint32, offset int64) error {
	return binary.Write(struct{ io.Writer }{io.NewOffsetWriter(f, offset)}, binary.LittleEndian, val)
}
