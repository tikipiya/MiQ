package voids

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	miq "github.com/tikipiya/MiQ"
)

type recordedCall struct {
	Path, Method string
	Body         map[string]any
}

func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	parsed, _ := url.Parse(server.URL + "/")
	zero := time.Duration(0)
	retries := 0
	client, err := NewClient(Options{BaseURL: parsed, HTTPClient: server.Client(), Retries: &retries, RetryDelay: &zero, AllowPrivateNetwork: true})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, server
}

func TestEndpointSelectionAndPayload(t *testing.T) {
	var mu sync.Mutex
	calls := []recordedCall{}
	png := []byte{0x89, 0x50, 0x4e, 0x47}
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		mu.Lock()
		calls = append(calls, recordedCall{r.URL.Path, r.Method, body})
		mu.Unlock()
		if r.URL.Path == HostedEndpoint {
			_ = json.NewEncoder(w).Encode(map[string]string{"url": "http://" + r.Host + "/q.png"})
			return
		}
		w.Write(png)
	})
	defer server.Close()
	avatar, _ := url.Parse("https://example.test/a.png")
	quote := Quote{Text: "Hello", Avatar: avatar, Username: "cat", DisplayName: "Cat", Color: true, Watermark: "MIQ"}
	address, err := client.HostedURL(context.Background(), quote)
	if err != nil {
		t.Fatal(err)
	}
	if address.Path != "/q.png" {
		t.Fatalf("url=%s", address)
	}
	direct, err := client.Direct(context.Background(), quote)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(direct, png) {
		t.Fatalf("direct=%x", direct)
	}
	hosted, err := client.HostedBytes(context.Background(), quote)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hosted, png) {
		t.Fatalf("hosted=%x", hosted)
	}
	mu.Lock()
	defer mu.Unlock()
	paths := []string{}
	for _, call := range calls {
		paths = append(paths, call.Path)
	}
	want := []string{HostedEndpoint, DirectEndpoint, HostedEndpoint, "/q.png"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v want=%v", paths, want)
	}
	payload := calls[0].Body
	if payload["display_name"] != "Cat" || payload["avatar"] != "https://example.test/a.png" || payload["color"] != true {
		t.Fatalf("payload=%#v", payload)
	}
}

func TestHostedResponseValidation(t *testing.T) {
	for name, response := range map[string]string{"not-json": "<html>", "missing-url": "{\"ok\":true}", "relative-url": "{\"url\":\"/q.png\"}"} {
		t.Run(name, func(t *testing.T) {
			client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, response) })
			defer server.Close()
			_, err := client.HostedURL(context.Background(), Quote{Text: "hi"})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Endpoint != HostedEndpoint {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestHTTPErrorCarriesContractDetails(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, `{"message":"nope"}`)
	})
	defer server.Close()
	_, err := client.Direct(context.Background(), Quote{Text: "hi"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error=%v", err)
	}
	if apiErr.Status != 503 || apiErr.Endpoint != DirectEndpoint || string(apiErr.Body) != `{"message":"nope"}` || !errors.Is(err, miq.ErrAPI) {
		t.Fatalf("api error=%#v", apiErr)
	}
}

func TestRetryAndHeaders(t *testing.T) {
	attempts := 0
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		authorization = r.Header.Get("Authorization")
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte("png"))
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	retries := 2
	zero := time.Duration(0)
	client, err := NewClient(Options{BaseURL: base, HTTPClient: server.Client(), Retries: &retries, RetryDelay: &zero, Headers: http.Header{"Authorization": {"Bearer token"}}, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Direct(context.Background(), Quote{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "png" || attempts != 3 || authorization != "Bearer token" {
		t.Fatalf("got=%q attempts=%d auth=%q", got, attempts, authorization)
	}
}

func TestCancellationAndTimeout(t *testing.T) {
	t.Run("canceled", func(t *testing.T) {
		client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		defer server.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Direct(ctx, Quote{Text: "hi"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
		defer server.Close()
		base, _ := url.Parse(server.URL)
		zero := 0
		client, err := NewClient(Options{BaseURL: base, HTTPClient: server.Client(), Timeout: 10 * time.Millisecond, Retries: &zero, AllowPrivateNetwork: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Direct(context.Background(), Quote{Text: "hi"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestInputValidationAndLimits(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("12345")) })
	defer server.Close()
	if _, err := client.Direct(context.Background(), Quote{}); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("blank error=%v", err)
	}
	bad, _ := url.Parse("file:///avatar.png")
	if _, err := client.Direct(context.Background(), Quote{Text: "hi", Avatar: bad}); !errors.Is(err, miq.ErrValidation) {
		t.Fatalf("avatar error=%v", err)
	}
	base, _ := url.Parse(server.URL)
	zero := 0
	limited, err := NewClient(Options{BaseURL: base, HTTPClient: server.Client(), Retries: &zero, MaxResponseBytes: 4, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limited.Direct(context.Background(), Quote{Text: "hi"}); !errors.Is(err, miq.ErrAPI) {
		t.Fatalf("limit error=%v", err)
	}
}

func TestPrivateNetworkRejectedByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("png")) }))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	zero := 0
	client, err := NewClient(Options{BaseURL: base, HTTPClient: server.Client(), Retries: &zero})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Direct(context.Background(), Quote{Text: "hi"})
	if !errors.Is(err, miq.ErrAPI) {
		t.Fatalf("error=%v", err)
	}
}
