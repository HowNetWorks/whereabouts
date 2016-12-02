package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Source struct {
	url *url.URL
}

func (s *Source) String() string {
	return s.url.String()
}

func NewSource(urlString string) (*Source, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "file", "http", "https":
		return &Source{url: u}, nil
	}

	return nil, fmt.Errorf("unknown scheme %q", u.Scheme)
}

func (s *Source) Read() ([]byte, error) {
	switch s.url.Scheme {
	case "file":
		return ioutil.ReadFile(s.url.Path)
	case "http", "https":
		resp, err := http.Get(s.url.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, err
		}

		return ioutil.ReadAll(resp.Body)
	}

	return nil, fmt.Errorf("unknown scheme %q", s.url.Scheme)
}
