// Package runtime controls Docker and Podman through host-local Unix sockets.
package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
)

type Container struct {
	ID, Image string
	Running   bool
	ExitCode  int
}
type CreateRequest struct {
	RunID, RunnerID, Image, Workspace, MountPath string
	ReadOnly                                     bool
	Spec                                         jobs.Spec
	Environment                                  []string
}
type Runtime interface {
	Pull(context.Context, string) (string, error)
	Build(context.Context, string, string, string, string) (string, error)
	Create(context.Context, CreateRequest) (string, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Logs(context.Context, string, io.Writer) error
	Stats(context.Context, string) (protocol.Metrics, error)
	Inspect(context.Context, string) (Container, error)
	List(context.Context, string) ([]Container, error)
	Remove(context.Context, string) error
	RemoveImage(context.Context, string) error
}
type transport struct {
	client *http.Client
	kind   string
	socket string
}
type Docker struct{ *transport }
type Podman struct{ *transport }

func NewDocker(socket string) (*Docker, error) {
	t, err := newTransport(socket, "docker")
	return &Docker{t}, err
}
func NewPodman(socket string) (*Podman, error) {
	t, err := newTransport(socket, "podman")
	return &Podman{t}, err
}
func New(kind, socket string) (Runtime, error) {
	switch kind {
	case "docker":
		return NewDocker(socket)
	case "podman":
		return NewPodman(socket)
	default:
		return nil, errors.New("runtime must be docker or podman")
	}
}
func newTransport(socket, kind string) (*transport, error) {
	if !filepath.IsAbs(socket) {
		return nil, errors.New("runtime socket must be an absolute Unix socket path")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socket)
	}, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second}
	return &transport{client: &http.Client{Transport: tr}, kind: kind, socket: socket}, nil
}
func (t *transport) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.HasPrefix(path, "/build") {
		req.Header.Set("Content-Type", "application/x-tar")
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, errors.New("runtime socket request failed")
	}
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && (method != http.MethodDelete || resp.StatusCode != http.StatusNotFound) {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s runtime returned HTTP %d", t.kind, resp.StatusCode)
	}
	return resp, nil
}
func (t *transport) json(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	resp, err := t.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<20))
	return err
}
func (t *transport) Pull(ctx context.Context, ref string) (string, error) {
	resp, err := t.request(ctx, "POST", "/images/create?fromImage="+url.QueryEscape(ref), nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if err = progress(resp.Body); err != nil {
		return "", err
	}
	return t.image(ctx, ref)
}
func (t *transport) image(ctx context.Context, ref string) (string, error) {
	var value struct {
		ID string `json:"Id"`
	}
	err := t.json(ctx, "GET", "/images/"+url.PathEscape(ref)+"/json", nil, &value)
	return value.ID, err
}
func progress(r io.Reader) error {
	decoder := json.NewDecoder(r)
	for {
		var v struct {
			Error string `json:"error"`
		}
		err := decoder.Decode(&v)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if v.Error != "" {
			return errors.New("runtime image operation failed")
		}
	}
}
func (t *transport) Build(ctx context.Context, workspace, contextPath, dockerfile, tag string) (string, error) {
	if !strings.HasPrefix(tag, "gocicle-run-") || !jobs.ValidName(tag) {
		return "", errors.New("build tag must identify a gocicle run")
	}
	root, err := Resolve(workspace, contextPath)
	if err != nil {
		return "", err
	}
	file, err := Resolve(workspace, dockerfile)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, file)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", errors.New("the Dockerfile must remain within the build context")
	}
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() { err := archive(root, writer); _ = writer.CloseWithError(err); done <- err }()
	defer func() { _ = reader.Close() }()
	query := url.Values{"t": {tag}, "dockerfile": {filepath.ToSlash(relative)}, "nocache": {"1"}, "rm": {"1"}, "forcerm": {"1"}}
	// Podman and Docker both expose this compatibility endpoint. Each adapter has independent conformance tests.
	resp, err := t.request(ctx, "POST", "/build?"+query.Encode(), reader)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	err = progress(resp.Body)
	_ = reader.Close()
	archiveErr := <-done
	if err != nil || archiveErr != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = t.RemoveImage(cleanup, tag)
		return "", errors.Join(err, archiveErr)
	}
	return t.image(ctx, tag)
}
func (t *transport) Create(ctx context.Context, r CreateRequest) (string, error) {
	if strings.ContainsAny(r.Workspace+r.MountPath, ":,\n\x00") {
		return "", errors.New("mount paths contain unsupported characters")
	}
	socket, err := filepath.EvalSymlinks(t.socket)
	if err != nil {
		return "", err
	}
	workspace, err := filepath.EvalSymlinks(r.Workspace)
	if err != nil {
		return "", err
	}
	if ContainsPath(workspace, socket) {
		return "", errors.New("job mounts must exclude the runtime socket")
	}
	bind := r.Workspace + ":" + r.MountPath
	mode := "rw"
	if r.ReadOnly {
		mode = "ro"
	}
	if t.kind == "podman" {
		mode += ",Z"
	}
	bind += ":" + mode
	host := map[string]any{"Binds": []string{bind}, "NanoCpus": int64(r.Spec.Limits.CPUs * 1e9), "Memory": r.Spec.Limits.Memory, "MemorySwap": r.Spec.Limits.Memory, "PidsLimit": r.Spec.Limits.PIDs, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges"}, "AutoRemove": false, "LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "10m", "max-file": "1"}}}
	if t.kind == "podman" {
		host["UsernsMode"] = "keep-id"
	}
	req := map[string]any{"Image": r.Image, "User": fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "Entrypoint": []string{r.Spec.Command.Path}, "Cmd": r.Spec.Command.Args, "WorkingDir": r.Spec.Command.WorkingDirectory, "Env": r.Environment, "AttachStdout": true, "AttachStderr": true, "Tty": false, "HostConfig": host, "Labels": map[string]string{"gocicle.run": r.RunID, "gocicle.runner": r.RunnerID}}
	var result struct {
		ID string `json:"Id"`
	}
	err = t.json(ctx, "POST", "/containers/create?name=gocicle-"+url.QueryEscape(r.RunID), req, &result)
	return result.ID, err
}
func (t *transport) Start(ctx context.Context, id string) error {
	return t.json(ctx, "POST", "/containers/"+url.PathEscape(id)+"/start", nil, nil)
}
func (t *transport) Stop(ctx context.Context, id string) error {
	return t.json(ctx, "POST", "/containers/"+url.PathEscape(id)+"/stop?t=5", nil, nil)
}
func (t *transport) Remove(ctx context.Context, id string) error {
	return t.json(ctx, "DELETE", "/containers/"+url.PathEscape(id)+"?force=1&v=1", nil, nil)
}
func (t *transport) RemoveImage(ctx context.Context, id string) error {
	return t.json(ctx, "DELETE", "/images/"+url.PathEscape(id)+"?force=1", nil, nil)
}
func (t *transport) Inspect(ctx context.Context, id string) (Container, error) {
	var v struct {
		ID    string `json:"Id"`
		Image string
		State struct {
			Running  bool
			ExitCode int
		}
	}
	err := t.json(ctx, "GET", "/containers/"+url.PathEscape(id)+"/json", nil, &v)
	return Container{ID: v.ID, Image: v.Image, Running: v.State.Running, ExitCode: v.State.ExitCode}, err
}
func (t *transport) List(ctx context.Context, runner string) ([]Container, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {"gocicle.runner=" + runner}})
	var rows []struct {
		ID      string `json:"Id"`
		ImageID string
	}
	err := t.json(ctx, "GET", "/containers/json?all=1&filters="+url.QueryEscape(string(filters)), nil, &rows)
	result := make([]Container, 0, len(rows))
	for _, r := range rows {
		result = append(result, Container{ID: r.ID, Image: r.ImageID})
	}
	return result, err
}
func (t *transport) Logs(ctx context.Context, id string, w io.Writer) error {
	return t.logs(ctx, id, w, true)
}
func (t *transport) logs(ctx context.Context, id string, w io.Writer, follow bool) error {
	resp, err := t.request(ctx, "GET", "/containers/"+url.PathEscape(id)+"/logs?follow="+strconv.FormatBool(follow)+"&stdout=1&stderr=1&timestamps=1", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	for {
		var header [8]byte
		_, err := io.ReadFull(resp.Body, header[:])
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		n := binary.BigEndian.Uint32(header[4:])
		if n > 16<<20 {
			return errors.New("runtime log frame exceeds the limit")
		}
		if _, err = io.CopyN(w, resp.Body, int64(n)); err != nil {
			return err
		}
	}
}
func (t *transport) Stats(ctx context.Context, id string) (protocol.Metrics, error) {
	type cpu struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		System uint64 `json:"system_cpu_usage"`
		Online uint64 `json:"online_cpus"`
	}
	var v struct {
		CPU      cpu `json:"cpu_stats"`
		Previous cpu `json:"precpu_stats"`
		Memory   struct {
			Usage uint64 `json:"usage"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RX uint64 `json:"rx_bytes"`
			TX uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}
	m := protocol.Metrics{At: time.Now()}
	if err := t.json(ctx, "GET", "/containers/"+url.PathEscape(id)+"/stats?stream=false", nil, &v); err != nil {
		return m, err
	}
	m.Available = true
	m.Memory = &v.Memory.Usage
	if v.CPU.System > v.Previous.System && v.CPU.CPUUsage.TotalUsage >= v.Previous.CPUUsage.TotalUsage && v.CPU.Online > 0 {
		cpu := float64(v.CPU.CPUUsage.TotalUsage-v.Previous.CPUUsage.TotalUsage) / float64(v.CPU.System-v.Previous.System) * float64(v.CPU.Online) * 100
		m.CPU = &cpu
	}
	if v.Networks != nil {
		var rx, tx uint64
		for _, net := range v.Networks {
			rx += net.RX
			tx += net.TX
		}
		m.NetworkRX = &rx
		m.NetworkTX = &tx
	}
	return m, nil
}
func Resolve(root, relative string) (string, error) {
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(base, relative))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("resolved path leaves the approved root")
	}
	return target, nil
}
func archive(root string, w io.Writer) error {
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = scoped.Close() }()
	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()
	err = fs.WalkDir(scoped.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return errors.New("build context must contain only regular files and directories")
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if err = tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := scoped.Open(name)
		if err != nil {
			return err
		}
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
			_ = f.Close()
			return errors.New("build context changed during archiving")
		}
		_, copyErr := io.Copy(tw, f)
		return errors.Join(copyErr, f.Close())
	})
	return errors.Join(err, tw.Close())
}

// ContainsPath reports whether target equals root or is a path below root.
func ContainsPath(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
