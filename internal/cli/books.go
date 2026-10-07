package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	zlib "github.com/heartleo/zlib"
	"github.com/heartleo/zlib/internal/config"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func bookListClient(cmd *cobra.Command, requireSession bool) (*zlib.Client, error) {
	if endpointFlag(cmd, "backend") == "html" {
		return nil, fmt.Errorf("%s requires EAPI; use --backend auto or eapi", cmd.Name())
	}
	session, err := config.LoadSession()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("could not load session: %w", err)
	}
	if requireSession && (session.Cookies["remix_userid"] == "" || session.Cookies["remix_userkey"] == "") {
		return nil, fmt.Errorf("%w; run `zlib login`", zlib.ErrNotLoggedIn)
	}
	return configureClient(cmd, session)
}

func addBookListFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output books as a JSON array")
	cmd.Flags().Bool("full-title", false, "Do not truncate book titles in the results table")
}

// commandBooks copies books before replacing IDs with CLI download references.
func commandBooks(books []zlib.Book) []zlib.Book {
	out := make([]zlib.Book, len(books))
	copy(out, books)
	for i, book := range out {
		if id := bookCommandID(book); id != "" {
			out[i].ID = id
		}
	}
	return out
}

func writeBookList(cmd *cobra.Command, books []zlib.Book) error {
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(commandBooks(books))
	}
	if len(books) == 0 {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s No books found\n", colorRed(symbolError))
		return err
	}
	fullTitle, _ := cmd.Flags().GetBool("full-title")
	fmt.Fprintf(cmd.OutOrStdout(), "%s Found %s\n\n", colorGreen(symbolSuccess), colorBold(fmt.Sprintf("%d results", len(books))))
	return renderBooks(cmd.OutOrStdout(), books, fullTitle)
}

// renderBooks is the shared static table for search, popular, and recommended.
func renderBooks(w io.Writer, books []zlib.Book, fullTitle bool) error {
	return renderBookTable(w, books, fullTitle, bookCommandID)
}

func renderSearchBooks(w io.Writer, books []zlib.Book, fullTitle bool) error {
	return renderBookTable(w, books, fullTitle, searchDisplayID)
}

func renderBookTable(w io.Writer, books []zlib.Book, fullTitle bool, displayID func(zlib.Book) string) error {
	rows := make([][]string, 0, len(books))
	for i, b := range books {
		id := displayID(b)
		if id == "" {
			id = "-"
		}
		authors := strings.Join(b.Authors, ", ")
		title := b.Name
		if !fullTitle {
			title = runewidth.Truncate(title, 40, "")
		}
		ext := strings.ToUpper(b.Extension)
		if ext == "" {
			ext = "-"
		}
		size := b.Size
		if size == "" {
			size = "-"
		}
		year := b.Year
		if year == "" || year == "0" {
			year = "-"
		}
		rating := b.Rating
		if rating == "" {
			rating = "-"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1), id,
			title,
			runewidth.Truncate(authors, 20, ""),
			year, ext, size, rating,
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(currentTheme.Accent)).
		Headers("#", "ID", "Title", "Authors", "Year", "Format", "Size", "Rating").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			pad := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
			if row == table.HeaderRow {
				return pad.Bold(true).Foreground(currentTheme.Accent)
			}
			switch col {
			case 1: // ID
				return pad.Foreground(currentTheme.Link)
			case 2: // Title
				return pad.Foreground(currentTheme.Title)
			case 5: // Format
				return pad.Foreground(extLipglossColor(rows[row][5]))
			case 7: // Rating
				return pad.Foreground(currentTheme.Success)
			case 0, 3, 4, 6: // #, Authors, Year, Size
				return pad.Foreground(currentTheme.Muted)
			}
			return pad
		})
	_, err := fmt.Fprintln(w, t)
	return err
}
