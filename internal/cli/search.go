package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	zlib "github.com/heartleo/zlib"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for books",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()

		jsonOut, _ := cmd.Flags().GetBool("json")

		if len(args) == 0 {
			if jsonOut {
				return fmt.Errorf("--json requires a search query (interactive mode has no JSON output)")
			}
			fullTitle, _ := cmd.Flags().GetBool("full-title")
			selectedID, err := interactiveSearch(c, searchOptsFromFlags(cmd), fullTitle)
			if err != nil {
				return err
			}
			if selectedID == "" {
				return nil
			}
			dir, err := destDirFlag(cmd)
			if err != nil {
				return err
			}
			sendToKindle, _ := cmd.Flags().GetBool("send-to-kindle")

			return runDownload(newDownloadModel(selectedID, dir, c), sendToKindle)
		}

		query := strings.Join(args, " ")
		page, _ := cmd.Flags().GetInt("page")
		count, _ := cmd.Flags().GetInt("count")
		fullTitle, _ := cmd.Flags().GetBool("full-title")
		opts := searchOptsFromFlags(cmd)

		result, err := c.Search(query, page, count, opts)
		if err != nil {
			return err
		}

		if jsonOut {
			result.Books = commandBooks(result.Books)
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		}

		if len(result.Books) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s No results found for %q\n", colorRed(symbolError), query)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s Found %s · Page %d / %d\n\n",
			colorGreen(symbolSuccess),
			colorBold(fmt.Sprintf("%d results", len(result.Books))),
			result.Page, result.TotalPages,
		)

		return renderSearchBooks(cmd.OutOrStdout(), result.Books, fullTitle)
	},
}

// bookIDFromURL extracts the alphanumeric book ID from a URL like
// https://z-library.sk/book/2RAqApzDRL/title.html
func bookIDFromURL(u string) string {
	parts := strings.Split(u, "/")
	for i, p := range parts {
		if p == "book" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return u
}

// searchDisplayID keeps the EAPI ID separate from the download reference.
func searchDisplayID(book zlib.Book) string {
	if book.ID != "" {
		return book.ID
	}
	return bookCommandID(book)
}

func bookCommandID(book zlib.Book) string {
	if book.ID != "" && book.Hash != "" {
		return book.ID + ":" + book.Hash
	}
	id := bookIDFromURL(book.URL)
	if id == book.URL {
		return ""
	}
	return id
}

func historyCommandID(item zlib.DownloadHistoryItem) string {
	if item.ID != "" && item.Hash != "" {
		return item.ID + ":" + item.Hash
	}
	id := bookIDFromURL(item.URL)
	if id == item.URL {
		return ""
	}
	return id
}

func searchOptsFromFlags(cmd *cobra.Command) *zlib.SearchOptions {
	exts, _ := cmd.Flags().GetStringArray("ext")
	formats, _ := cmd.Flags().GetStringArray("format")
	exts = append(exts, formats...)
	if len(exts) == 0 {
		return nil
	}
	seen := make(map[zlib.Extension]bool)
	var extensions []zlib.Extension
	for _, e := range exts {
		ext := zlib.Extension(strings.ToUpper(e))
		if seen[ext] {
			continue
		}
		seen[ext] = true
		extensions = append(extensions, ext)
	}
	return &zlib.SearchOptions{Extensions: extensions}
}

func init() {
	searchCmd.Flags().IntP("page", "p", 1, "Page number")
	searchCmd.Flags().IntP("count", "n", 50, "Results per page")
	addDestDirFlag(searchCmd)
	searchCmd.Flags().Bool("send-to-kindle", false, "Send the downloaded file to Kindle.")
	searchCmd.Flags().StringArray("ext", nil, "Filter by file extension (repeatable): epub, pdf, mobi, …")
	searchCmd.Flags().StringArray("format", nil, "Alias for --ext: filter by file format (repeatable)")
	searchCmd.Flags().Bool("full-title", false, "Do not truncate book titles in the results table")
	searchCmd.Flags().Bool("json", false, "Output results as JSON (requires a query)")
}
