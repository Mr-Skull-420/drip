package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"drip/internal/client/tcp"
	"drip/internal/shared/protocol"
	"drip/internal/shared/utils"
	"drip/pkg/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	httpsSubdomain    string
	httpsDaemonMode   bool
	httpsDaemonMarker bool
	httpsLocalAddress string
)

var httpsCmd = &cobra.Command{
	Use:   "https <port>",
	Short: "Start HTTPS tunnel",
	Long: `Start an HTTPS tunnel to expose a local HTTPS server.

Example:
  drip https 443                    Tunnel localhost:443
  drip https 8443 --subdomain myapp Use custom subdomain

Configuration:
  First time: Run 'drip config init' to save server and token
  Subsequent: Just run 'drip https <port>'

Note: Uses TCP over TLS 1.3 for secure communication`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTPS,
}

func init() {
	httpsCmd.Flags().StringVarP(&httpsSubdomain, "subdomain", "n", "", "Custom subdomain (optional)")
	httpsCmd.Flags().BoolVarP(&httpsDaemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	httpsCmd.Flags().StringVarP(&httpsLocalAddress, "address", "a", "127.0.0.1", "Local address to forward to (default: 127.0.0.1)")
	httpsCmd.Flags().BoolVar(&httpsDaemonMarker, "daemon-child", false, "Internal flag for daemon child process")
	httpsCmd.Flags().MarkHidden("daemon-child")
	rootCmd.AddCommand(httpsCmd)
}

func runHTTPS(cmd *cobra.Command, args []string) error {
	// Parse port
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	// Handle daemon mode
	if httpsDaemonMode && !httpsDaemonMarker {
		// Start as daemon
		daemonArgs := append([]string{"https"}, args...)
		daemonArgs = append(daemonArgs, "--daemon-child")
		if httpsSubdomain != "" {
			daemonArgs = append(daemonArgs, "--subdomain", httpsSubdomain)
		}
		if httpsLocalAddress != "127.0.0.1" {
			daemonArgs = append(daemonArgs, "--address", httpsLocalAddress)
		}
		if serverURL != "" {
			daemonArgs = append(daemonArgs, "--server", serverURL)
		}
		if authToken != "" {
			daemonArgs = append(daemonArgs, "--token", authToken)
		}
		if insecure {
			daemonArgs = append(daemonArgs, "--insecure")
		}
		if verbose {
			daemonArgs = append(daemonArgs, "--verbose")
		}
		return StartDaemon("https", port, daemonArgs)
	}

	// Initialize logger
	if err := utils.InitLogger(verbose); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer utils.Sync()

	logger := utils.GetLogger()

	// Load configuration or use command line flags
	var serverAddr, token string

	if serverURL == "" {
		// Try to load from config file
		cfg, err := config.LoadClientConfig("")
		if err != nil {
			return fmt.Errorf(`configuration not found.

Please run 'drip config init' first, or use flags:
  drip https %d --server SERVER:PORT --token TOKEN`, port)
		}
		serverAddr = cfg.Server
		token = cfg.Token
	} else {
		// Use command line flags
		serverAddr = serverURL
		token = authToken
	}

	// Validate server address
	if serverAddr == "" {
		return fmt.Errorf("server address is required")
	}

	// Create connector config
	connConfig := &tcp.ConnectorConfig{
		ServerAddr: serverAddr,
		Token:      token,
		TunnelType: protocol.TunnelTypeHTTPS,
		LocalHost:  httpsLocalAddress,
		LocalPort:  port,
		Subdomain:  httpsSubdomain,
		Insecure:   insecure,
	}

	// Setup signal handler for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Connection loop with reconnect support
	reconnectAttempts := 0
	for {
		// Create connector
		connector := tcp.NewConnector(connConfig, logger)

		// Connect to server
		if reconnectAttempts == 0 {
			fmt.Printf("\033[36m🔌 Connecting to %s...\033[0m\n", serverAddr)
		} else {
			fmt.Printf("\033[33m🔄 Reconnecting to %s (attempt %d/%d)...\033[0m\n", serverAddr, reconnectAttempts, maxReconnectAttempts)
		}

		if err := connector.Connect(); err != nil {
			// Check if this is a non-retryable error
			if isNonRetryableError(err) {
				return fmt.Errorf("failed to connect: %w", err)
			}

			reconnectAttempts++
			if reconnectAttempts >= maxReconnectAttempts {
				return fmt.Errorf("failed to connect after %d attempts: %w", maxReconnectAttempts, err)
			}
			fmt.Printf("\033[31m✗ Connection failed: %v\033[0m\n", err)
			fmt.Printf("\033[90m  Retrying in %v...\033[0m\n", reconnectInterval)

			// Wait before retry, but allow interrupt
			select {
			case <-quit:
				fmt.Println("\n\033[33m🛑 Shutting down...\033[0m")
				return nil
			case <-time.After(reconnectInterval):
				continue
			}
		}

		// Reset reconnect attempts on successful connection
		reconnectAttempts = 0

		// Save daemon info if running as daemon child
		if httpsDaemonMarker {
			daemonInfo := &DaemonInfo{
				PID:        os.Getpid(),
				Type:       "https",
				Port:       port,
				Subdomain:  httpsSubdomain,
				Server:     serverAddr,
				URL:        connector.GetURL(),
				StartTime:  time.Now(),
				Executable: os.Args[0],
			}
			if err := SaveDaemonInfo(daemonInfo); err != nil {
				// Log but don't fail
				logger.Warn("Failed to save daemon info", zap.Error(err))
			}
		}

		// Print tunnel information
		fmt.Println()
		fmt.Println("\033[1;32m╔══════════════════════════════════════════════════════════════════╗\033[0m")
		fmt.Println("\033[1;32m║\033[0m              \033[1;37m🔒 HTTPS Tunnel Connected Successfully!\033[0m             \033[1;32m║\033[0m")
		fmt.Println("\033[1;32m╠══════════════════════════════════════════════════════════════════╣\033[0m")
		fmt.Printf("\033[1;32m║\033[0m  \033[1;37mTunnel URL:\033[0m                                                   \033[1;32m║\033[0m\n")
		fmt.Printf("\033[1;32m║\033[0m  \033[1;36m%-60s\033[0m \033[1;32m║\033[0m\n", connector.GetURL())
		fmt.Println("\033[1;32m║\033[0m                                                                  \033[1;32m║\033[0m")
		displayAddr := httpsLocalAddress
		if displayAddr == "127.0.0.1" {
			displayAddr = "localhost"
		}
		fmt.Printf("\033[1;32m║\033[0m  \033[90mForwarding:\033[0m \033[1m%s:%d\033[0m → \033[36m%s\033[0m%-15s\033[1;32m║\033[0m\n", displayAddr, port, "public", "")
		fmt.Printf("\033[1;32m║\033[0m  \033[90mLatency:\033[0m    \033[90mmeasuring...\033[0m%-40s\033[1;32m║\033[0m\n", "")
		fmt.Printf("\033[1;32m║\033[0m  \033[90mTraffic:\033[0m    \033[90m↓ 0 B  ↑ 0 B\033[0m%-32s\033[1;32m║\033[0m\n", "")
		fmt.Printf("\033[1;32m║\033[0m  \033[90mSpeed:\033[0m      \033[90m↓ 0 B/s  ↑ 0 B/s\033[0m%-28s\033[1;32m║\033[0m\n", "")
		fmt.Printf("\033[1;32m║\033[0m  \033[90mRequests:\033[0m   \033[90m0\033[0m%-43s\033[1;32m║\033[0m\n", "")
		fmt.Println("\033[1;32m╠══════════════════════════════════════════════════════════════════╣\033[0m")
		fmt.Println("\033[1;32m║\033[0m  \033[90mPress Ctrl+C to stop the tunnel\033[0m                              \033[1;32m║\033[0m")
		fmt.Println("\033[1;32m╚══════════════════════════════════════════════════════════════════╝\033[0m")
		fmt.Println()

		// Setup latency display
		latencyCh := make(chan time.Duration, 1)
		connector.SetLatencyCallback(func(latency time.Duration) {
			select {
			case latencyCh <- latency:
			default:
			}
		})

		// Start stats display updater (updates every second)
		stopDisplay := make(chan struct{})
		disconnected := make(chan struct{})

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			var lastLatency time.Duration
			for {
				select {
				case latency := <-latencyCh:
					lastLatency = latency
				case <-ticker.C:
					// Update speed calculation
					stats := connector.GetStats()
					if stats != nil {
						stats.UpdateSpeed()
						snapshot := stats.GetSnapshot()

						// Move cursor up 8 lines to update display
						fmt.Print("\033[8A")

						// Update latency line
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mLatency:\033[0m    %s%-40s\033[1;32m║\033[0m\n", formatLatency(lastLatency), "")

						// Update traffic line
						trafficStr := fmt.Sprintf("↓ %s  ↑ %s", tcp.FormatBytes(snapshot.TotalBytesIn), tcp.FormatBytes(snapshot.TotalBytesOut))
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mTraffic:\033[0m    \033[36m%-48s\033[0m\033[1;32m║\033[0m\n", trafficStr)

						// Update speed line
						speedStr := fmt.Sprintf("↓ %s  ↑ %s", tcp.FormatSpeed(snapshot.SpeedIn), tcp.FormatSpeed(snapshot.SpeedOut))
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mSpeed:\033[0m      \033[33m%-48s\033[0m\033[1;32m║\033[0m\n", speedStr)

						// Update requests line
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mRequests:\033[0m   \033[35m%-47d\033[0m\033[1;32m║\033[0m\n", snapshot.TotalRequests)

						// Move back down 4 lines
						fmt.Print("\033[4B")
					}
				case <-stopDisplay:
					return
				}
			}
		}()

		// Monitor connection in background
		go func() {
			connector.Wait()
			close(disconnected)
		}()

		// Wait for signal or disconnection
		select {
		case <-quit:
			close(stopDisplay)
			fmt.Println("\n\n\033[33m🛑 Shutting down...\033[0m")
			connector.Close()
			if httpsDaemonMarker {
				RemoveDaemonInfo("https", port)
			}
			fmt.Println("\033[32m✓\033[0m Tunnel closed")
			return nil
		case <-disconnected:
			close(stopDisplay)
			fmt.Println("\n\n\033[31m⚠ Connection lost!\033[0m")
			reconnectAttempts++
			if reconnectAttempts >= maxReconnectAttempts {
				return fmt.Errorf("connection lost after %d reconnect attempts", maxReconnectAttempts)
			}
			fmt.Printf("\033[90m  Reconnecting in %v...\033[0m\n", reconnectInterval)

			// Wait before reconnect, but allow interrupt
			select {
			case <-quit:
				fmt.Println("\n\033[33m🛑 Shutting down...\033[0m")
				return nil
			case <-time.After(reconnectInterval):
				continue
			}
		}
	}
}
