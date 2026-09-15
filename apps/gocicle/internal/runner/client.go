package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
)

type Client struct {
	URL, Token string
	HTTP       *http.Client
}

func NewClient(origin, token string) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("API URL must use HTTPS")
	}
	return &Client{URL: strings.TrimRight(origin, "/"), Token: token, HTTP: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Call(ctx context.Context, method, path, lease string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	if lease != "" {
		req.Header.Set("X-Gocicle-Lease", lease)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("control plane request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("control plane rejected the request")
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
	}
	return nil
}

type spoolMetadata struct {
	RunID, Lease string
	Deadline     time.Time
}
type Spool struct {
	mu       sync.Mutex
	dir      string
	sequence int64
	events   []protocol.Event
	bytes    int
	client   *Client
	metadata spoolMetadata
}

func NewSpool(dir string, c *Client, runID, lease string) (*Spool, error) {
	path := filepath.Join(dir, "spool-"+runID)
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, err
	}
	s := &Spool{dir: path, client: c, metadata: spoolMetadata{RunID: runID, Lease: lease, Deadline: time.Now().Add(24 * time.Hour)}}
	raw, _ := json.Marshal(s.metadata)
	if err := os.WriteFile(filepath.Join(path, "metadata.json"), raw, 0600); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Spool) file(sequence int64) string {
	return filepath.Join(s.dir, fmt.Sprintf("%020d.json", sequence))
}
func (s *Spool) Add(kind string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > 64<<10 || s.bytes+len(data) > 12<<20 {
		return errors.New("runner event spool is full")
	}
	next := s.sequence + 1
	e := protocol.Event{Sequence: next, Kind: kind, Data: data}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := s.file(next)
	file, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- The path contains an internal sequence number under the private spool directory.
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return err
	}
	s.sequence = next
	s.events = append(s.events, e)
	s.bytes += len(data)
	return nil
}
func (s *Spool) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.events) > 0 {
		e := s.events[0]
		if err := s.client.Call(ctx, "POST", "/api/v1/runner/runs/"+s.metadata.RunID+"/events", s.metadata.Lease, e, nil); err != nil {
			return err
		}
		if err := os.Remove(s.file(e.Sequence)); err != nil {
			return err
		}
		s.bytes -= len(e.Data)
		s.events = s.events[1:]
	}
	return nil
}
func (s *Spool) Empty() bool  { s.mu.Lock(); defer s.mu.Unlock(); return len(s.events) == 0 }
func (s *Spool) Close() error { return os.RemoveAll(s.dir) }
func RecoverSpools(ctx context.Context, dir string, c *Client) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "spool-") {
			continue
		}
		s := &Spool{dir: filepath.Join(dir, entry.Name()), client: c}
		data, err := os.ReadFile(filepath.Join(s.dir, "metadata.json"))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &s.metadata); err != nil {
			return err
		}
		if time.Now().After(s.metadata.Deadline) {
			if err := s.Close(); err != nil {
				return err
			}
			continue
		}
		files, err := os.ReadDir(s.dir)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.Name() == "metadata.json" || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(s.dir, file.Name()))
			if err != nil {
				return err
			}
			var event protocol.Event
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			s.events = append(s.events, event)
			s.bytes += len(event.Data)
			if s.bytes > 12<<20 {
				return errors.New("stored runner spool exceeds its limit")
			}
		}
		sort.Slice(s.events, func(i, j int) bool { return s.events[i].Sequence < s.events[j].Sequence })
		if err := s.Flush(ctx); err == nil {
			if err := s.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}
