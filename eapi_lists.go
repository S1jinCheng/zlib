package zlib

import "fmt"

const (
	eapiPopularPath     = "/eapi/book/most-popular"
	eapiRecommendedPath = "/eapi/user/book/recommended"
)

// Popular returns the current most-popular books through EAPI. A session is
// optional; the server controls anonymous access and which metadata is returned.
func (c *Client) Popular() ([]Book, error) {
	return c.eapiBookList(eapiPopularPath, false)
}

// Recommended returns personalized EAPI recommendations using the current
// remix credentials. Both list methods use EAPI even for an HTML-mode client.
func (c *Client) Recommended() ([]Book, error) {
	return c.eapiBookList(eapiRecommendedPath, true)
}

func (c *Client) eapiBookList(path string, requireSession bool) ([]Book, error) {
	return runOperation(c, c.ctx, func(d *Client) ([]Book, error) {
		d.mode = ClientModeEAPI
		if requireSession {
			cookies := d.cookieSnapshot()
			if cookies["remix_userid"] == "" || cookies["remix_userkey"] == "" {
				return nil, ErrNotLoggedIn
			}
		}
		body, err := d.eapiGet(path, nil)
		if err != nil {
			return nil, err
		}
		var result struct {
			Books []eapiBook `json:"books"`
		}
		if err := decodeEAPIJSON(body, &result); err != nil {
			return nil, fmt.Errorf("%w: invalid EAPI book list: %v", ErrEndpointResponse, err)
		}
		if result.Books == nil {
			return nil, fmt.Errorf("%w: missing books in EAPI book list", ErrEndpointResponse)
		}
		books := make([]Book, 0, len(result.Books))
		for _, item := range result.Books {
			books = append(books, d.mapEAPIBook(item))
		}
		return books, nil
	}, nil)
}
