package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"drip/pkg/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  "Manage Drip client configuration (server, token, etc.)",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration interactively",
	Long:  "Initialize Drip configuration with interactive prompts",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  "Display the current Drip configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration values",
	Long:  "Set specific configuration values (server, token)",
	RunE:  runConfigSet,
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration",
	Long:  "Delete the configuration file",
	RunE:  runConfigReset,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  "Validate the configuration file",
	RunE:  runConfigValidate,
}

var (
	configFull   bool
	configForce  bool
	configServer string
	configToken  string
)

func init() {
	// Add subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configValidateCmd)

	// Flags for show
	configShowCmd.Flags().BoolVar(&configFull, "full", false, "Show full token (not hidden)")

	// Flags for set
	configSetCmd.Flags().StringVar(&configServer, "server", "", "Server address (e.g., tunnel.example.com:443)")
	configSetCmd.Flags().StringVar(&configToken, "token", "", "Authentication token")

	// Flags for reset
	configResetCmd.Flags().BoolVar(&configForce, "force", false, "Force reset without confirmation")

	// Add to root
	rootCmd.AddCommand(configCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	fmt.Println("\n╔═══════════════════════════════════════╗")
	fmt.Println("║  Drip Configuration Setup             ║")
	fmt.Println("╚═══════════════════════════════════════╝")

	reader := bufio.NewReader(os.Stdin)

	// Get server address
	fmt.Print("Server address (e.g., tunnel.example.com:443): ")
	serverAddr, _ := reader.ReadString('\n')
	serverAddr = strings.TrimSpace(serverAddr)

	if serverAddr == "" {
		return fmt.Errorf("server address is required")
	}

	// Get token
	fmt.Print("Authentication token (leave empty to skip): ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	// Create config
	cfg := &config.ClientConfig{
		Server: serverAddr,
		Token:  token,
		TLS:    true,
	}

	// Save config
	if err := config.SaveClientConfig(cfg, ""); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("\n✓ Configuration saved to", config.DefaultClientConfigPath())
	fmt.Println("✓ You can now use 'drip' without --server and --token")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadClientConfig("")
	if err != nil {
		return err
	}

	fmt.Println("\n╔═══════════════════════════════════════╗")
	fmt.Println("║  Current Configuration                 ║")
	fmt.Println("╚═══════════════════════════════════════╝")

	fmt.Printf("Server:  %s\n", cfg.Server)

	// Show token (hidden or full)
	if cfg.Token != "" {
		if configFull {
			fmt.Printf("Token:   %s\n", cfg.Token)
		} else {
			// Hide middle part of token
			if len(cfg.Token) > 10 {
				fmt.Printf("Token:   %s***%s (hidden)\n",
					cfg.Token[:3],
					cfg.Token[len(cfg.Token)-3:],
				)
			} else {
				fmt.Printf("Token:   %s (hidden)\n", cfg.Token[:3]+"***")
			}
		}
	} else {
		fmt.Println("Token:   (not set)")
	}

	fmt.Printf("TLS:     %s\n", enabledDisabled(cfg.TLS))
	fmt.Printf("Config:  %s\n\n", config.DefaultClientConfigPath())

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	// Load existing config or create new
	cfg, err := config.LoadClientConfig("")
	if err != nil {
		// Create new config if not exists
		cfg = &config.ClientConfig{
			TLS: true,
		}
	}

	// Update fields if provided
	modified := false

	if configServer != "" {
		cfg.Server = configServer
		modified = true
		fmt.Printf("✓ Server updated: %s\n", configServer)
	}

	if configToken != "" {
		cfg.Token = configToken
		modified = true
		fmt.Println("✓ Token updated")
	}

	if !modified {
		return fmt.Errorf("no changes specified. Use --server or --token")
	}

	// Save config
	if err := config.SaveClientConfig(cfg, ""); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("✓ Configuration saved")

	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	configPath := config.DefaultClientConfigPath()

	// Check if config exists
	if !config.ConfigExists("") {
		fmt.Println("No configuration file found")
		return nil
	}

	// Confirm deletion
	if !configForce {
		fmt.Print("Are you sure you want to delete the configuration? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete config file
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}

	fmt.Println("✓ Configuration file deleted")

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	fmt.Println("\nValidating configuration...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Load config
	cfg, err := config.LoadClientConfig("")
	if err != nil {
		fmt.Println("✗ Failed to load configuration")
		return err
	}

	// Validate server address
	if cfg.Server == "" {
		fmt.Println("✗ Server address is not set")
		return fmt.Errorf("invalid configuration")
	}
	fmt.Println("✓ Server address is valid")

	// Validate token
	if cfg.Token != "" {
		fmt.Println("✓ Token is set")
	} else {
		fmt.Println("⚠ Token is not set (authentication may fail)")
	}

	// Validate TLS
	if cfg.TLS {
		fmt.Println("✓ TLS is enabled")
	} else {
		fmt.Println("⚠ TLS is disabled (not recommended for production)")
	}

	fmt.Println("\n✓ Configuration is valid")

	return nil
}

func enabledDisabled(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}
