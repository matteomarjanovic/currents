package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type pinterestRoundTripFunc func(*http.Request) (*http.Response, error)

func (f pinterestRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResolvePinterestShortURL(t *testing.T) {
	original := pinterestHTTP
	defer func() { pinterestHTTP = original }()
	pinterestHTTP = &http.Client{Transport: pinterestRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var status int
		var location string
		switch req.URL.Hostname() {
		case "pin.it":
			status, location = http.StatusPermanentRedirect, "https://api.pinterest.com/url_shortener/abc/redirect/"
		case "api.pinterest.com":
			status, location = http.StatusFound, "https://www.pinterest.com/giova_merlo/consolle/?invite_code=abc"
		case "www.pinterest.com":
			status = http.StatusOK
		default:
			return nil, fmt.Errorf("unexpected host %q", req.URL.Hostname())
		}
		header := make(http.Header)
		if location != "" {
			header.Set("Location", location)
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})}

	got, err := resolvePinterestUsername(context.Background(), "https://pin.it/abc")
	if err != nil || got != "giova_merlo" {
		t.Fatalf("resolvePinterestUsername() = %q, %v; want giova_merlo", got, err)
	}
}

func TestResolvePinterestShortURLRejectsExternalRedirect(t *testing.T) {
	original := pinterestHTTP
	defer func() { pinterestHTTP = original }()
	pinterestHTTP = &http.Client{Transport: pinterestRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "pin.it" {
			t.Fatalf("unexpected request to %s", req.URL.Hostname())
		}
		header := make(http.Header)
		header.Set("Location", "https://example.com/elsewhere")
		return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})}

	if _, err := resolvePinterestUsername(context.Background(), "https://pin.it/abc"); err == nil {
		t.Fatal("external redirect unexpectedly accepted")
	}
}

func TestNormalizePinterestUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "username", input: " giova_merlo ", want: "giova_merlo"},
		{name: "at username", input: "@giova_merlo", want: "giova_merlo"},
		{name: "profile URL", input: "https://www.pinterest.com/giova_merlo/", want: "giova_merlo"},
		{name: "localized board URL", input: "https://it.pinterest.com/giova_merlo/consolle/.", want: "giova_merlo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePinterestUsername(tt.input)
			if err != nil {
				t.Fatalf("normalizePinterestUsername() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizePinterestUsername() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePinterestUsernameRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "https://example.com/giova_merlo/", "giova_merlo/consolle"} {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizePinterestUsername(input); err == nil {
				t.Fatalf("normalizePinterestUsername(%q) unexpectedly succeeded", input)
			}
		})
	}
}
