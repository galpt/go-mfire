package mfire

import "sync"

// This file provides package-level convenience functions so callers don't have
// to manage a Client instance when the defaults are acceptable.

var (
	defaultClient *Client
	defaultOnce   sync.Once
)

func defaultC() *Client {
	defaultOnce.Do(func() {
		defaultClient = NewClient()
	})
	return defaultClient
}

// Home returns the top `limit` manga from the MangaFire home page using a
// package-level default client. The function is thread-safe.
func Home(limit int) ([]Manga, error) {
	return defaultC().FetchHome(limit)
}

// Search performs a site search for `query` and returns up to `limit` results
// using the package-level default client. The function is thread-safe.
func Search(query string, limit int) ([]Manga, error) {
	return defaultC().Search(query, limit)
}

// GetDefaultClient returns the package-level client instance. Callers who
// require custom configuration can construct their own Client via NewClient().
func GetDefaultClient() *Client {
	return defaultC()
}
