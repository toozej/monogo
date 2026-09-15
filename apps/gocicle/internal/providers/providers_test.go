package providers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestTangledRepositoryIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"records":[{"uri":"at://did:plc:user/sh.tangled.repo/stable-key","value":{"name":"Display name","knot":"knot.example.com"}},{"uri":"at://did:plc:user/sh.tangled.repo/new-key","value":{"knot":"knot.example.com","repoDid":"did:plc:repository"}}]}`)
	}))
	defer server.Close()
	a, err := New("tangled", Config{}, "")
	if err != nil {
		t.Fatal(err)
	}
	a.http = server.Client()
	token := (&oauth2.Token{}).WithExtra(map[string]any{"subject": "did:plc:user", "pds": server.URL})
	repos, err := a.Repositories(context.Background(), token, "")
	if err != nil || len(repos) != 2 {
		t.Fatalf("repositories=%+v error=%v", repos, err)
	}
	if repos[0].URL != "https://knot.example.com/did:plc:user/stable-key" || repos[1].SSHURL != "ssh://git@knot.example.com/did:plc:repository" {
		t.Fatal("repository transport used a cosmetic name instead of stable identity")
	}
}

func TestSourceHutMissingIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"me":null}}`)
	}))
	defer server.Close()
	a, err := New("sourcehut", Config{APIURL: server.URL}, "")
	if err != nil {
		t.Fatal(err)
	}
	a.http = server.Client()
	if _, err := a.profile(context.Background(), &oauth2.Token{AccessToken: "fixture"}); err == nil {
		t.Fatal("missing SourceHut subject was accepted")
	}
}

func TestLoginAndRepositoryAdapters(t *testing.T) {
	for _, kind := range []string{"github", "gitlab", "forgejo", "codeberg", "sourcehut"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.Contains(r.URL.Path, "access_token") || strings.Contains(r.URL.Path, "access-token") || r.URL.Path == "/oauth/token":
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if r.Form.Get("code_verifier") == "" {
						t.Error("PKCE verifier was omitted")
					}
					_, _ = io.WriteString(w, `{"access_token":"access","token_type":"Bearer"}`)
				case strings.HasSuffix(r.URL.Path, "/user"):
					if r.Header.Get("Authorization") != "Bearer access" {
						t.Error("access token was omitted")
					}
					_, _ = io.WriteString(w, `{"id":123,"login":"user","name":"User","email":"user@example.com"}`)
				case r.URL.Path == "/query":
					var body struct {
						Query string `json:"query"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if strings.Contains(body.Query, "repositories") {
						_, _ = io.WriteString(w, `{"data":{"repositories":{"results":[{"name":"repo","owner":{"canonicalName":"~user"}}]}}}`)
					} else {
						_, _ = io.WriteString(w, `{"data":{"me":{"id":123,"username":"user","email":"user@example.com"}}}`)
					}
				default:
					_, _ = io.WriteString(w, `[{"name":"repo","clone_url":"https://example.com/repo.git","http_url_to_repo":"https://example.com/repo.git","default_branch":"main"}]`)
				}
			}))
			defer server.Close()
			a, err := New(kind, Config{BaseURL: server.URL, APIURL: server.URL, ClientID: "client", RedirectURL: "https://gocicle.example/auth/callback"}, "secret")
			if err != nil {
				t.Fatal(err)
			}
			a.http = server.Client()
			location, state, err := a.Begin(context.Background(), "state", "")
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(location)
			if err != nil {
				t.Fatal(err)
			}
			if u.Query().Get("state") != "state" || u.Query().Get("code_challenge_method") != "S256" {
				t.Fatal("login did not bind state and PKCE")
			}
			profile, token, err := a.Exchange(context.Background(), "code", "", state)
			if err != nil || profile.Subject != "123" {
				t.Fatalf("profile=%+v err=%v", profile, err)
			}
			repos, err := a.Repositories(context.Background(), token, "")
			if err != nil || len(repos) != 1 {
				t.Fatalf("repos=%+v err=%v", repos, err)
			}
		})
	}
}
func TestDPoPProof(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := DPoP(base64.RawURLEncoding.EncodeToString(der), "POST", "https://auth.example/token?x=1", "server-nonce", "access")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatal("invalid JWT")
	}
	header, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if !strings.Contains(string(header), `"typ":"dpop+jwt"`) {
		t.Fatal("wrong DPoP type")
	}
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(payload, &claims)
	if claims["nonce"] != "server-nonce" || claims["htu"] != "https://auth.example/token" || claims["htm"] != "POST" || claims["ath"] == nil {
		t.Fatalf("claims=%+v", claims)
	}
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(&key.PublicKey, sum[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		t.Fatal("invalid DPoP signature")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestATProtocolDiscoveryPARAndIssuerBinding(t *testing.T) {
	a, err := New("tangled", Config{ClientID: "https://gocicle.example/oauth-client-metadata.json", RedirectURL: "https://gocicle.example/auth/tangled/callback"}, "")
	if err != nil {
		t.Fatal(err)
	}
	parAttempts := 0
	a.http = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := ""
		status := 200
		headers := http.Header{"Content-Type": {"application/json"}}
		switch r.URL.Host + r.URL.Path {
		case "plc.directory/did:plc:account":
			body = `{"id":"did:plc:account","service":[{"id":"#atproto_pds","type":"AtprotoPersonalDataServer","serviceEndpoint":"https://pds.example"}]}`
		case "pds.example/.well-known/oauth-protected-resource":
			body = `{"resource":"https://pds.example","authorization_servers":["https://auth.example"]}`
		case "auth.example/.well-known/oauth-authorization-server":
			body = `{"issuer":"https://auth.example","authorization_endpoint":"https://auth.example/authorize","token_endpoint":"https://auth.example/token","pushed_authorization_request_endpoint":"https://auth.example/par","code_challenge_methods_supported":["S256"],"dpop_signing_alg_values_supported":["ES256"]}`
		case "auth.example/par":
			parAttempts++
			if r.Header.Get("DPoP") == "" {
				t.Error("PAR omitted DPoP")
			}
			if parAttempts == 1 {
				status = 400
				headers.Set("DPoP-Nonce", "nonce")
				body = `{"error":"use_dpop_nonce"}`
			} else {
				body = `{"request_uri":"urn:request:123","expires_in":90}`
			}
		case "auth.example/token":
			if r.Header.Get("DPoP") == "" {
				t.Error("token request omitted DPoP")
			}
			body = `{"access_token":"access","token_type":"DPoP","sub":"did:plc:account","scope":"atproto","expires_in":300}`
		default:
			t.Errorf("unexpected request: %s", r.URL)
			status = 404
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	location, state, err := a.Begin(context.Background(), "state", "did:plc:account")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(location, "request_uri=urn%3Arequest%3A123") || parAttempts != 2 {
		t.Fatal("PAR nonce negotiation failed")
	}
	if _, _, err := a.Exchange(context.Background(), "code", "https://attacker.example", state); err == nil {
		t.Fatal("callback issuer mismatch was accepted")
	}
	profile, _, err := a.Exchange(context.Background(), "code", "https://auth.example", state)
	if err != nil || profile.Subject != "did:plc:account" {
		t.Fatalf("profile=%+v error=%v", profile, err)
	}
}
func TestGenericGitHasNoLogin(t *testing.T) {
	a, err := New("git", Config{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Begin(context.Background(), "state", ""); err == nil {
		t.Fatal("generic Git offered OAuth")
	}
	if _, err := a.Repositories(context.Background(), (*oauth2.Token)(nil), ""); err == nil {
		t.Fatal("discovery accepted a missing connection")
	}
}
