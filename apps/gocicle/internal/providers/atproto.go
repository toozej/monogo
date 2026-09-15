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
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"golang.org/x/oauth2"
)

type authorizationServer struct {
	Issuer         string   `json:"issuer"`
	Authorization  string   `json:"authorization_endpoint"`
	Token          string   `json:"token_endpoint"`
	PAR            string   `json:"pushed_authorization_request_endpoint"`
	CodeChallenges []string `json:"code_challenge_methods_supported"`
	DPoPAlgorithms []string `json:"dpop_signing_alg_values_supported"`
}
type didDocument struct {
	ID          string   `json:"id"`
	AlsoKnownAs []string `json:"alsoKnownAs"`
	Service     []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Endpoint string `json:"serviceEndpoint"`
	} `json:"service"`
}

func (a *Adapter) resolve(ctx context.Context, input string) (string, string, error) {
	did := input
	handle := ""
	if !strings.HasPrefix(input, "did:") {
		if strings.ContainsAny(input, "/:?#@ ") || !strings.Contains(input, ".") {
			return "", "", errors.New("AT Protocol handle is invalid")
		}
		handle = input
		records, _ := net.DefaultResolver.LookupTXT(ctx, "_atproto."+handle)
		for _, record := range records {
			if strings.HasPrefix(record, "did=") {
				if did != input {
					return "", "", errors.New("handle has multiple DID records")
				}
				did = strings.TrimPrefix(record, "did=")
			}
		}
		if did == input {
			req, err := http.NewRequestWithContext(ctx, "GET", "https://"+handle+"/.well-known/atproto-did", nil)
			if err != nil {
				return "", "", err
			}
			resp, err := a.do(req)
			if err != nil {
				return "", "", errors.New("cannot resolve AT Protocol handle")
			}
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2048))
			_ = resp.Body.Close()
			if readErr != nil || resp.StatusCode != 200 {
				return "", "", errors.New("cannot resolve AT Protocol handle")
			}
			did = strings.TrimSpace(string(body))
		}
	}
	var endpoint string
	switch {
	case strings.HasPrefix(did, "did:plc:"):
		suffix := strings.TrimPrefix(did, "did:plc:")
		if suffix == "" || strings.ContainsAny(suffix, "/?#:") {
			return "", "", errors.New("PLC DID is invalid")
		}
		endpoint = "https://plc.directory/" + did
	case strings.HasPrefix(did, "did:web:"):
		host := strings.TrimPrefix(did, "did:web:")
		if host == "" || strings.ContainsAny(host, "/:?#%@") {
			return "", "", errors.New("did:web must identify a hostname")
		}
		endpoint = "https://" + host + "/.well-known/did.json"
	default:
		return "", "", errors.New("AT Protocol DID method is unsupported")
	}
	var doc didDocument
	if err := a.request(ctx, "GET", endpoint, "", nil, &doc); err != nil {
		return "", "", err
	}
	if doc.ID != did {
		return "", "", errors.New("DID document identity does not match")
	}
	if handle != "" {
		linked := false
		for _, alias := range doc.AlsoKnownAs {
			if alias == "at://"+handle {
				linked = true
			}
		}
		if !linked {
			return "", "", errors.New("DID document does not confirm the handle")
		}
	}
	for _, service := range doc.Service {
		if strings.HasSuffix(service.ID, "#atproto_pds") && service.Type == "AtprotoPersonalDataServer" {
			if err := httpsURL(service.Endpoint); err != nil {
				return "", "", err
			}
			return did, strings.TrimRight(service.Endpoint, "/"), nil
		}
	}
	return "", "", errors.New("DID document has no AT Protocol PDS")
}
func (a *Adapter) discover(ctx context.Context, pds string) (authorizationServer, error) {
	var resource struct {
		Resource string   `json:"resource"`
		Servers  []string `json:"authorization_servers"`
	}
	if err := a.request(ctx, "GET", pds+"/.well-known/oauth-protected-resource", "", nil, &resource); err != nil {
		return authorizationServer{}, err
	}
	if resource.Resource != pds || len(resource.Servers) != 1 {
		return authorizationServer{}, errors.New("PDS OAuth metadata is invalid")
	}
	issuer := resource.Servers[0]
	if err := httpsURL(issuer); err != nil {
		return authorizationServer{}, err
	}
	var server authorizationServer
	if err := a.request(ctx, "GET", strings.TrimRight(issuer, "/")+"/.well-known/oauth-authorization-server", "", nil, &server); err != nil {
		return server, err
	}
	if server.Issuer != issuer {
		return server, errors.New("OAuth issuer does not match discovery metadata")
	}
	for _, endpoint := range []string{server.Authorization, server.Token, server.PAR} {
		if err := httpsURL(endpoint); err != nil {
			return server, err
		}
	}
	if !has(server.CodeChallenges, "S256") || !has(server.DPoPAlgorithms, "ES256") {
		return server, errors.New("OAuth server must support S256 PKCE and ES256 DPoP")
	}
	return server, nil
}
func (a *Adapter) beginAT(ctx context.Context, state, hint string) (string, State, error) {
	did, pds, err := a.resolve(ctx, hint)
	if err != nil {
		return "", State{}, err
	}
	server, err := a.discover(ctx, pds)
	if err != nil {
		return "", State{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", State{}, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", State{}, err
	}
	s := State{Verifier: oauth2.GenerateVerifier(), DPoPKey: base64.RawURLEncoding.EncodeToString(der), Issuer: server.Issuer, PDS: pds, Subject: did, TokenURL: server.Token}
	challenge := sha256.Sum256([]byte(s.Verifier))
	form := url.Values{"client_id": {a.Config.ClientID}, "redirect_uri": {a.Config.RedirectURL}, "response_type": {"code"}, "scope": {"atproto"}, "state": {state}, "login_hint": {hint}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}}
	var result struct {
		URI     string `json:"request_uri"`
		Expires int    `json:"expires_in"`
	}
	if err := a.dpopForm(ctx, server.PAR, form, &s, &result); err != nil {
		return "", s, err
	}
	if result.URI == "" || result.Expires <= 0 {
		return "", s, errors.New("OAuth PAR response is invalid")
	}
	query := url.Values{"client_id": {a.Config.ClientID}, "request_uri": {result.URI}}
	return server.Authorization + "?" + query.Encode(), s, nil
}
func (a *Adapter) exchangeAT(ctx context.Context, code, issuer string, s State) (Profile, *oauth2.Token, error) {
	if issuer != s.Issuer {
		return Profile{}, nil, errors.New("OAuth callback issuer does not match")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {a.Config.ClientID}, "redirect_uri": {a.Config.RedirectURL}, "code_verifier": {s.Verifier}}
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Subject     string `json:"sub"`
		Scope       string `json:"scope"`
		Expires     int    `json:"expires_in"`
	}
	if err := a.dpopForm(ctx, s.TokenURL, form, &s, &response); err != nil {
		return Profile{}, nil, err
	}
	if response.Subject != s.Subject || !strings.EqualFold(response.TokenType, "DPoP") || !has(strings.Fields(response.Scope), "atproto") || response.AccessToken == "" {
		return Profile{}, nil, errors.New("AT Protocol token identity is invalid")
	}
	_, pds, err := a.resolve(ctx, response.Subject)
	if err != nil || pds != s.PDS {
		return Profile{}, nil, errors.New("AT Protocol PDS changed during login")
	}
	server, err := a.discover(ctx, pds)
	if err != nil || server.Issuer != s.Issuer {
		return Profile{}, nil, errors.New("AT Protocol issuer changed during login")
	}
	token := (&oauth2.Token{AccessToken: response.AccessToken, TokenType: response.TokenType, Expiry: time.Now().Add(time.Duration(response.Expires) * time.Second)}).WithExtra(map[string]any{"subject": s.Subject, "pds": s.PDS, "dpopKey": s.DPoPKey})
	return Profile{Subject: s.Subject, Name: s.Subject}, token, nil
}
func (a *Adapter) dpopForm(ctx context.Context, endpoint string, form url.Values, s *State, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		proof, err := DPoP(s.DPoPKey, "POST", endpoint, s.Nonce, "")
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", proof)
		resp, err := a.do(req)
		if err != nil {
			return errors.New("AT Protocol OAuth request failed")
		}
		nonce := resp.Header.Get("DPoP-Nonce")
		if nonce != "" {
			s.Nonce = nonce
		}
		if (resp.StatusCode == 400 || resp.StatusCode == 401) && nonce != "" && attempt == 0 {
			_ = resp.Body.Close()
			continue
		}
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			_ = resp.Body.Close()
			return errors.New("AT Protocol OAuth request was rejected")
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
		_ = resp.Body.Close()
		return err
	}
	return errors.New("AT Protocol DPoP nonce negotiation failed")
}
func DPoP(encoded, method, endpoint, nonce, token string) (string, error) {
	der, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	raw, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", err
	}
	key, ok := raw.(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New("DPoP key is not an EC key")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	u.RawQuery = ""
	u.Fragment = ""
	header := map[string]any{"typ": "dpop+jwt", "alg": "ES256", "jwk": map[string]string{"kty": "EC", "crv": "P-256", "x": coordinate(key.X), "y": coordinate(key.Y)}}
	claims := map[string]any{"jti": security.Token(), "htm": method, "htu": u.String(), "iat": time.Now().Unix()}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if token != "" {
		sum := sha256.Sum256([]byte(token))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	h, _ := json.Marshal(header)
	p, _ := json.Marshal(claims)
	body := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	sum := sha256.Sum256([]byte(body))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return body + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
func coordinate(i *big.Int) string {
	b := make([]byte, 32)
	i.FillBytes(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func has(values []string, match string) bool {
	for _, value := range values {
		if value == match {
			return true
		}
	}
	return false
}
func (a *Adapter) repositoriesAT(ctx context.Context, token *oauth2.Token, cursor string) ([]Repository, error) {
	subject, _ := token.Extra("subject").(string)
	pds, _ := token.Extra("pds").(string)
	if subject == "" || pds == "" {
		return nil, errors.New("the Tangled connection needs an AT Protocol identity")
	}
	query := url.Values{"repo": {subject}, "collection": {"sh.tangled.repo"}, "limit": {"100"}}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var records struct {
		Records []struct {
			URI   string                               `json:"uri"`
			Value struct{ Name, Knot, RepoDid string } `json:"value"`
		} `json:"records"`
	}
	if err := a.request(ctx, "GET", pds+"/xrpc/com.atproto.repo.listRecords?"+query.Encode(), "", nil, &records); err != nil {
		return nil, err
	}
	result := []Repository{}
	for _, r := range records.Records {
		if strings.ContainsAny(r.Value.Knot, "/:?#@") || r.Value.Knot == "" {
			return nil, errors.New("the Tangled knot hostname is invalid")
		}
		prefix := "at://" + subject + "/sh.tangled.repo/"
		if !strings.HasPrefix(r.URI, prefix) {
			return nil, errors.New("repository record does not belong to the authenticated identity")
		}
		key := strings.TrimPrefix(r.URI, prefix)
		if key == "" || key == "." || key == ".." || strings.ContainsAny(key, "/?#\\\x00") {
			return nil, errors.New("repository record key is invalid")
		}
		target := r.Value.RepoDid
		if target == "" {
			target = subject + "/" + key
		} else if !strings.HasPrefix(target, "did:") || strings.ContainsAny(target, "/?#\\\x00") {
			return nil, errors.New("repository DID is invalid")
		}
		name := r.Value.Name
		if name == "" {
			name = key
		}
		result = append(result, Repository{Name: name, URL: "https://" + r.Value.Knot + "/" + target, SSHURL: "ssh://git@" + r.Value.Knot + "/" + target})
	}
	return result, nil
}
func ClientMetadata(publicURL, callback string) map[string]any {
	return map[string]any{"client_id": publicURL + "/oauth-client-metadata.json", "client_name": "gocicle", "client_uri": publicURL, "redirect_uris": []string{callback}, "scope": "atproto", "grant_types": []string{"authorization_code"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none", "dpop_bound_access_tokens": true}
}
