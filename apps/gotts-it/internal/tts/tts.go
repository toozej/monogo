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

// maxWAVHeaderBytes bounds metadata preceding the audio data. Audio itself is
// streamed, so the response size is limited by the request context rather
// than buffered in memory.
const maxWAVHeaderBytes = 1 << 20

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
	var wavFormat []byte
	for i, chunkText := range chunks {
		log.WithField("chunk", fmt.Sprintf("%d/%d", i+1, len(chunks))).Debugf("synthesizing %d chars", len([]rune(chunkText)))

		chunkCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		resp, err := synthesizeChunk(chunkCtx, client, chunkText, opts, format)
		if err != nil {
			cancel()
			return fmt.Errorf("synthesize chunk %d/%d: %w", i+1, len(chunks), err)
		}

		wavFormat, err = writeChunk(f, resp, opts.Format, i == 0, i == len(chunks)-1, wavFormat)
		if err != nil {
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

	if opts.Format == "wav" {
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

func writeChunk(f *os.File, resp *http.Response, format string, isFirst, isLast bool, expectedWAVFormat []byte) ([]byte, error) {
	switch format {
	case "mp3", "pcm":
		_, err := io.Copy(f, resp.Body)
		return expectedWAVFormat, err
	case "wav":
		return writeWAVChunk(f, resp.Body, isFirst, expectedWAVFormat)
	case "flac":
		if !isFirst {
			return expectedWAVFormat, fmt.Errorf("FLAC chunked synthesis not yet supported; use mp3 or wav")
		}
		_, err := io.Copy(f, resp.Body)
		return expectedWAVFormat, err
	default:
		_, err := io.Copy(f, resp.Body)
		return expectedWAVFormat, err
	}
}

func writeWAVChunk(f *os.File, r io.Reader, isFirst bool, expectedFormat []byte) ([]byte, error) {
	fixedHeader := make([]byte, 12)
	if _, err := io.ReadFull(r, fixedHeader); err != nil {
		return expectedFormat, fmt.Errorf("read WAV header: %w", err)
	}
	if !bytes.Equal(fixedHeader[:4], []byte("RIFF")) || !bytes.Equal(fixedHeader[8:12], []byte("WAVE")) {
		return expectedFormat, fmt.Errorf("invalid WAV response")
	}
	riffSize := int64(binary.LittleEndian.Uint32(fixedHeader[4:8]))
	if riffSize < 4 {
		return expectedFormat, fmt.Errorf("invalid WAV RIFF size")
	}
	remaining := riffSize - 4 // RIFF size includes the four-byte WAVE identifier.
	preDataBytes := int64(len(fixedHeader))
	var outputHeader bytes.Buffer
	if isFirst {
		_, _ = outputHeader.Write(fixedHeader)
	}
	var format []byte
	for remaining >= 8 {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(r, chunkHeader); err != nil {
			return expectedFormat, fmt.Errorf("read WAV chunk header: %w", err)
		}
		remaining -= 8
		size := int64(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		paddedSize := size + size%2
		if paddedSize > remaining {
			return expectedFormat, fmt.Errorf("invalid WAV chunk size")
		}
		if bytes.Equal(chunkHeader[:4], []byte("data")) {
			if len(format) == 0 {
				return expectedFormat, fmt.Errorf("WAV response has no format chunk before audio data")
			}
			if expectedFormat != nil && !bytes.Equal(format, expectedFormat) {
				return expectedFormat, fmt.Errorf("WAV chunk format does not match the first chunk")
			}
			blockAlign := int64(binary.LittleEndian.Uint16(format[12:14]))
			if size%blockAlign != 0 {
				return expectedFormat, fmt.Errorf("WAV audio data ends with a partial sample: %d bytes at block alignment %d", size, blockAlign)
			}
			if isFirst {
				if _, err := f.Write(outputHeader.Bytes()); err != nil {
					return expectedFormat, err
				}
				if _, err := f.Write(chunkHeader); err != nil {
					return expectedFormat, err
				}
			}
			if _, err := io.CopyN(f, r, size); err != nil {
				return expectedFormat, fmt.Errorf("read WAV audio data: %w", err)
			}
			if size%2 != 0 {
				if _, err := io.CopyN(io.Discard, r, 1); err != nil {
					return expectedFormat, fmt.Errorf("read WAV audio padding: %w", err)
				}
			}
			return format, nil
		}
		if preDataBytes+8+paddedSize > maxWAVHeaderBytes {
			return expectedFormat, fmt.Errorf("WAV header exceeds %d-byte limit", maxWAVHeaderBytes)
		}
		payload := make([]byte, paddedSize)
		if _, err := io.ReadFull(r, payload); err != nil {
			return expectedFormat, fmt.Errorf("read WAV chunk: %w", err)
		}
		if bytes.Equal(chunkHeader[:4], []byte("fmt ")) {
			if format != nil {
				return expectedFormat, fmt.Errorf("WAV response has duplicate format chunks")
			}
			format = append([]byte(nil), payload[:size]...)
			if err := validateWAVFormat(format); err != nil {
				return expectedFormat, err
			}
		}
		if isFirst {
			_, _ = outputHeader.Write(chunkHeader)
			_, _ = outputHeader.Write(payload)
		}
		preDataBytes += 8 + paddedSize
		remaining -= paddedSize
	}
	return expectedFormat, fmt.Errorf("WAV response has no data chunk")
}

func validateWAVFormat(format []byte) error {
	if len(format) < 16 {
		return fmt.Errorf("invalid WAV format chunk: got %d bytes, want at least 16", len(format))
	}
	encoding := binary.LittleEndian.Uint16(format[0:2])
	channels := binary.LittleEndian.Uint16(format[2:4])
	sampleRate := binary.LittleEndian.Uint32(format[4:8])
	byteRate := binary.LittleEndian.Uint32(format[8:12])
	blockAlign := binary.LittleEndian.Uint16(format[12:14])
	bitsPerSample := binary.LittleEndian.Uint16(format[14:16])
	if encoding == 0 || channels == 0 || sampleRate == 0 || byteRate == 0 || blockAlign == 0 || bitsPerSample == 0 {
		return fmt.Errorf("invalid WAV format chunk values")
	}
	if encoding != 1 && encoding != 3 {
		return fmt.Errorf("unsupported WAV encoding %d: chunked synthesis requires PCM or IEEE float audio", encoding)
	}
	wantBlockAlign := uint64(channels) * ((uint64(bitsPerSample) + 7) / 8)
	if uint64(blockAlign) != wantBlockAlign {
		return fmt.Errorf("invalid WAV block alignment %d for %d channels at %d bits", blockAlign, channels, bitsPerSample)
	}
	wantByteRate := uint64(sampleRate) * uint64(blockAlign)
	if uint64(byteRate) != wantByteRate {
		return fmt.Errorf("invalid WAV byte rate %d for sample rate %d and block alignment %d", byteRate, sampleRate, blockAlign)
	}
	return nil
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

	dataSize := fileSize - dataStart
	if dataSize%2 != 0 {
		if _, err := f.WriteAt([]byte{0}, fileSize); err != nil {
			return err
		}
		fileSize++
	}
	riffSize := fileSize - 8

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
