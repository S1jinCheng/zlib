package cli

import "github.com/spf13/cobra"

var popularCmd = &cobra.Command{
	Use:   "popular",
	Short: "Browse the most-popular books",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bookListClient(cmd, false)
		if err != nil {
			return err
		}
		books, err := c.Popular()
		if err != nil {
			return err
		}
		return writeBookList(cmd, books)
	},
}

func init() { addBookListFlags(popularCmd) }
