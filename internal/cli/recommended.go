package cli

import "github.com/spf13/cobra"

var recommendedCmd = &cobra.Command{
	Use:   "recommended",
	Short: "Browse personalized book recommendations (requires login)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := bookListClient(cmd, true)
		if err != nil {
			return err
		}
		books, err := c.Recommended()
		if err != nil {
			return err
		}
		return writeBookList(cmd, books)
	},
}

func init() { addBookListFlags(recommendedCmd) }
