package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/heartleo/zlib"
	"github.com/heartleo/zlib/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Z-Library",
	Long:  `Log in to Z-Library and save session cookies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		cookieFile, _ := cmd.Flags().GetString("cookies")
		if cookieFile != "" && (email != "" || password != "") {
			return fmt.Errorf("--cookies cannot be combined with --email or --password")
		}
		saved, _ := config.LoadSession()
		c, err := configureClient(cmd, config.Session{Domain: saved.Domain})
		if err != nil {
			return err
		}
		eapi, _ := cmd.Flags().GetBool("eapi")
		if eapi {
			if endpointFlag(cmd, "backend") == "html" {
				return fmt.Errorf("--eapi cannot be combined with --backend html")
			}
			c.SetMode(zlib.ClientModeEAPI)
		}
		domain := c.Domain()
		mode := string(c.Mode())
		if cookieFile != "" {
			count, err := importCookieSession(cookieFile, domain, time.Now(), mode)
			if err != nil {
				return fmt.Errorf("failed to import cookies: %w", err)
			}
			fmt.Printf("%s Imported %d cookies.\n", colorGreen(symbolSuccess), count)
			return nil
		}
		// Decide whether to prompt from the flags, before the remembered email
		// is filled in. Letting the prefill satisfy the check would make
		// `zlib login --password X` log into whatever account was remembered,
		// without ever showing the user which one.
		promptNeeded := email == "" || password == ""
		if email == "" {
			email = rememberedLoginEmail()
		}

		if promptNeeded {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Email").
						Value(&email),
					huh.NewInput().
						Title("Password").
						EchoMode(huh.EchoModePassword).
						Value(&password),
				),
			).WithTheme(huhTheme())
			if err := form.Run(); err != nil {
				return err
			}
		}

		if err := c.Login(email, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		if err := config.SaveSession(config.Session{
			Cookies: c.Cookies(),
			Domain:  c.Domain(),
			Mode:    string(c.Mode()),
		}); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}
		if err := rememberLoginEmail(email); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not remember login email: %v\n", err)
		}

		fmt.Printf("%s Logged in successfully.\n", colorGreen(symbolSuccess))
		return nil
	},
}

// rememberedLoginEmail returns the most recently successful login email. It
// is a convenience for the TUI only; passwords are never persisted.
func rememberedLoginEmail() string {
	cfg, err := config.LoadConfig()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.LastLoginEmail)
}

func rememberLoginEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}

	cfg, err := config.LoadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cfg.LastLoginEmail = email
	return config.SaveConfig(cfg)
}

func init() {
	loginCmd.Flags().String("email", "", "Z-Library email")
	loginCmd.Flags().String("password", "", "Z-Library password")
	loginCmd.Flags().String("cookies", "", "Import a Netscape-format cookie file")
	loginCmd.Flags().Bool("eapi", false, "Use EAPI (the default for new logins)")
}
