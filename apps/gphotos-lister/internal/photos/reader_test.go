package photos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

const testAlbum = "https://photos.google.com/share/album?key=secret"

type fakeSession struct {
	resolved    string
	navigateErr error
	rpc         func(string, []any) ([]any, error)
}

func (f fakeSession) Navigate(context.Context, string) (string, error) {
	return f.resolved, f.navigateErr
}
func (f fakeSession) RPC(_ context.Context, id string, payload []any) ([]any, error) {
	return f.rpc(id, payload)
}
func albumPage(items []any, next any) []any { return []any{nil, items, next, []any{"album"}} }
func heartEvent(id string, target float64) []any {
	return []any{float64(3), "event", float64(123), nil, nil, []any{[]any{"heart"}, target, []any{id}}}
}
func activity(events []any, next any) []any { return []any{next, events, "album"} }

func TestValidateAlbumURL(t *testing.T) {
	for _, raw := range []string{testAlbum, "https://photos.google.com/album/a", "https://photos.google.com/u/1/album/a", "https://photos.app.goo.gl/short"} {
		if err := ValidateAlbumURL(raw); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "http://photos.google.com/album/a", "https://photos.google.com.evil.test/album/a", "https://photos.google.com@evil.test/album/a", "https://user@photos.google.com/album/a", "https://photos.google.com:443/album/a", "https://photos.google.com/album/a/photo/b", "https://photos.google.com/", "https://photos.app.goo.gl/", "https://photos.app.goo.gl/a/b", "https://localhost/album/a", "https://photos.google.com/%zz"} {
		if err := ValidateAlbumURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestReadAllAlbumAndActivityPages(t *testing.T) {
	var calls []string
	session := fakeSession{resolved: testAlbum, rpc: func(id string, payload []any) ([]any, error) {
		calls = append(calls, id)
		switch id {
		case "snAcKc":
			if payload[0] != "album" || payload[3] != "secret" {
				t.Fatal(payload)
			}
			if payload[1] == nil {
				return albumPage([]any{[]any{"one"}, []any{"two"}, []any{"video", map[string]any{"76647426": []any{12.0}}}}, "page-2"), nil
			}
			if payload[1] != "page-2" {
				t.Fatal(payload)
			}
			return albumPage([]any{[]any{"two"}, []any{"three"}}, nil), nil
		case "wQ6iqd":
			if at(payload, 0, 0) != "album" || payload[2] != "secret" {
				t.Fatal(payload)
			}
			if payload[1] == nil {
				return activity([]any{heartEvent("one", 1), heartEvent("album", 2), heartEvent("video", 1), []any{float64(2), "comment"}}, "activity-2"), nil
			}
			if payload[1] != "activity-2" {
				t.Fatal(payload)
			}
			return activity([]any{heartEvent("three", 1), heartEvent("one", 1), heartEvent("removed-photo", 1)}, nil), nil
		case "fDcn4b":
			if payload[1] != nil || payload[2] != "secret" || at(payload, 5, 0) != 2 {
				t.Fatal(payload)
			}
			return []any{[]any{payload[0], nil, fmt.Sprint(payload[0]) + ".JPG"}}, nil
		default:
			t.Fatalf("unexpected RPC %s", id)
			return nil, nil
		}
	}}
	names, err := Read(context.Background(), session, testAlbum)
	if err != nil || !reflect.DeepEqual(names, []string{"one.JPG", "three.JPG"}) {
		t.Fatalf("%v %v", names, err)
	}
	if !reflect.DeepEqual(calls, []string{"snAcKc", "snAcKc", "wQ6iqd", "wQ6iqd", "fDcn4b", "fDcn4b"}) {
		t.Fatal(calls)
	}
}

func TestReadResolvesShortLinkAndAuthKey(t *testing.T) {
	metadata := make([]any, 20)
	metadata[0] = "resolved"
	metadata[19] = "album-auth"
	session := fakeSession{resolved: "https://photos.google.com/u/1/share/resolved", rpc: func(id string, payload []any) ([]any, error) {
		if id == "snAcKc" {
			return []any{nil, []any{[]any{"photo"}}, nil, metadata}, nil
		}
		if payload[2] != "album-auth" {
			t.Fatal(payload)
		}
		if id == "wQ6iqd" {
			return activity([]any{heartEvent("photo", 1)}, nil), nil
		}
		return []any{[]any{"photo", nil, "original.jpg"}}, nil
	}}
	names, err := Read(context.Background(), session, "https://photos.app.goo.gl/short")
	if err != nil || !reflect.DeepEqual(names, []string{"original.jpg"}) {
		t.Fatalf("%v %v", names, err)
	}
}

func TestReadFailures(t *testing.T) {
	expected := errors.New("network failed")
	for _, kind := range []string{"navigation", "sign-in", "unresolved-short", "rpc", "shape", "denied-album", "items", "item", "id", "page-token", "repeated-page", "activity-rpc", "filename-rpc", "filename-missing", "filename-invalid"} {
		t.Run(kind, func(t *testing.T) {
			session := fakeSession{resolved: testAlbum}
			if kind == "navigation" {
				session.navigateErr = expected
			}
			if kind == "sign-in" {
				session.resolved = "https://accounts.google.com/ServiceLogin"
			}
			if kind == "unresolved-short" {
				session.resolved = "https://photos.app.goo.gl/short"
			}
			session.rpc = func(id string, payload []any) ([]any, error) {
				if id == "wQ6iqd" {
					if kind == "activity-rpc" {
						return nil, expected
					}
					return activity([]any{heartEvent("p", 1)}, nil), nil
				}
				if id == "fDcn4b" {
					switch kind {
					case "filename-rpc":
						return nil, expected
					case "filename-missing":
						return []any{[]any{"p"}}, nil
					case "filename-invalid":
						return []any{[]any{"p", nil, "../bad"}}, nil
					}
					return []any{[]any{"p", nil, "good.jpg"}}, nil
				}
				switch kind {
				case "rpc":
					return nil, expected
				case "shape":
					return []any{nil}, nil
				case "denied-album":
					return []any{nil, nil, nil, nil}, nil
				case "items":
					return []any{nil, "bad", nil, []any{"album"}}, nil
				case "item":
					return albumPage([]any{"bad"}, nil), nil
				case "id":
					return albumPage([]any{[]any{nil}}, nil), nil
				case "page-token":
					return albumPage(nil, 1), nil
				case "repeated-page":
					return albumPage(nil, "again"), nil
				}
				return albumPage([]any{[]any{"p"}}, nil), nil
			}
			if _, err := Read(context.Background(), session, testAlbum); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, fakeSession{resolved: testAlbum}, testAlbum); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), fakeSession{}, "https://evil.test"); err == nil {
		t.Fatal("accepted external URL")
	}
}

func TestReadHeartValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []any
	}{
		{"shape", []any{}}, {"missing-album", []any{nil, nil, nil}}, {"events", []any{nil, "bad", "album"}}, {"event", activity([]any{"bad"}, nil)},
		{"type", activity([]any{[]any{"bad"}}, nil)}, {"target", activity([]any{[]any{float64(3)}}, nil)},
		{"unknown-target", activity([]any{heartEvent("p", 7)}, nil)}, {"missing-id", activity([]any{heartEvent("", 1)}, nil)},
		{"token", activity(nil, 17)}, {"repeated-token", activity(nil, "again")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := fakeSession{rpc: func(string, []any) ([]any, error) { return tc.data, nil }}
			if _, err := readHearts(context.Background(), session, "album", "key"); err == nil {
				t.Fatal("accepted invalid activity")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readHearts(ctx, fakeSession{}, "album", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestEmptyAndUnlikedAlbum(t *testing.T) {
	for _, empty := range []bool{false, true} {
		session := fakeSession{resolved: testAlbum, rpc: func(id string, _ []any) ([]any, error) {
			if id == "snAcKc" {
				if empty {
					return albumPage(nil, nil), nil
				}
				return albumPage([]any{[]any{"photo"}}, nil), nil
			}
			return activity(nil, nil), nil
		}}
		names, err := Read(context.Background(), session, testAlbum)
		if err != nil || len(names) != 0 {
			t.Fatalf("%v %v", names, err)
		}
	}
}

func TestAlbumHeartDoesNotSelectPhotos(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	session := fakeSession{resolved: testAlbum, rpc: func(id string, _ []any) ([]any, error) {
		switch id {
		case "snAcKc":
			return albumPage([]any{[]any{"one"}, []any{"two"}}, nil), nil
		case "wQ6iqd":
			return activity([]any{[]any{float64(1), "photo addition"}, heartEvent("album", 2)}, nil), nil
		default:
			t.Fatalf("unexpected filename request for an album heart: %s", id)
			return nil, nil
		}
	}}
	names, err := read(context.Background(), session, testAlbum, logger)
	if err != nil || len(names) != 0 {
		t.Fatalf("names = %v, error = %v", names, err)
	}
	for _, expected := range []string{"photos_total=2", "events=2", "album_hearts=1", "hearted_photo_ids_total=0", "matched_photos=0"} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("debug log does not contain %q: %s", expected, logs.String())
		}
	}
}

func rpcFrame(id string, data any) string {
	inner, _ := json.Marshal(data)
	outer, _ := json.Marshal([]any{[]any{"wrb.fr", id, string(inner), nil, nil, nil, "1"}})
	return ")]}'\n\n123\n" + string(outer) + "\n"
}
func TestDecodeRPC(t *testing.T) {
	data, err := decodeRPC(rpcFrame("snAcKc", []any{nil, []any{[]any{"photo"}}, nil, []any{"album"}}), "snAcKc")
	if err != nil || at(data, 1, 0, 0) != "photo" {
		t.Fatalf("%v %v", data, err)
	}
	for _, body := range []string{"", "<html>Sign in</html>", "[invalid]", `[["wrb.fr","snAcKc",null]]`, `[["wrb.fr","snAcKc",""]]`, `[["wrb.fr","snAcKc","invalid"]]`, `[["wrb.fr","snAcKc","null"]]`, rpcFrame("other", []any{}), `[["other","snAcKc","[]"]]`, `[[]]`} {
		if _, err := decodeRPC(body, "snAcKc"); err == nil {
			t.Errorf("accepted %q", body)
		}
	}
	if _, err := decodeRPC(strings.Repeat("x", maxResponseBytes+1), "x"); err == nil {
		t.Fatal("accepted oversized frame")
	}
	if at([]any{}, 2) != nil || at("text", 0) != nil {
		t.Fatal("invalid access")
	}
	if optional("") != nil || optional("key") != "key" {
		t.Fatal("optional key")
	}
}
