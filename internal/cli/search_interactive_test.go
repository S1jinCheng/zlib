package cli

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zlib "github.com/heartleo/zlib"
)

func TestSearchDisplaysIDAndKeepsDownloadReference(t *testing.T) {
	books := []zlib.Book{
		{ID: "11735659", Hash: "7659fb", Name: "Dune"},
		{ID: "123456789012", Hash: "abcdef", Name: "Another edition"},
	}
	m := searchSelectModel{state: searchStateResults, books: books}
	var table bytes.Buffer
	if err := renderSearchBooks(&table, books, false); err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{m.View(), table.String()} {
		for _, book := range books {
			if !strings.Contains(view, book.ID) {
				t.Errorf("search truncated ID %q", book.ID)
			}
			if strings.Contains(view, book.Hash) {
				t.Errorf("search displayed hash %q", book.Hash)
			}
		}
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(searchSelectModel).selectedID; got != "11735659:7659fb" {
		t.Fatalf("selected download reference = %q", got)
	}
	if got := commandBooks(books)[0].ID; got != "11735659:7659fb" {
		t.Fatalf("JSON download reference = %q", got)
	}
}

func TestHTMLSearchKeepsURLID(t *testing.T) {
	book := zlib.Book{URL: "https://z-lib.gl/book/QJW8QbLA3D/dune.html", Name: "Dune"}
	var out bytes.Buffer
	if err := renderSearchBooks(&out, []zlib.Book{book}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "QJW8QbLA3D") {
		t.Fatalf("HTML search ID missing: %s", out.String())
	}
	if got := bookCommandID(book); got != "QJW8QbLA3D" {
		t.Fatalf("HTML download reference = %q", got)
	}
}
