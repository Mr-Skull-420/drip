package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"drip/internal/client/tcp"
	"drip/internal/shared/protocol"
	"drip/internal/shared/utils"
	"drip/pkg/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	maxReconnectAttempts = 5
	reconnectInterval    = 3 * time.Second
)

var (
	subdomain    string
	daemonMode   bool
	daemonMarker bool
	localAddress string
)

var httpCmd = &cobra.Command{
	Use:   "http <port>",
	Short: "Start HTTP tunnel",
	Long: `Start an HTTP tunnel to expose a local HTTP server.

Example:
  drip http 3000                    Tunnel localhost:3000
  drip http 8080 --subdomain myapp  Use custom subdomain

Configuration:
  First time: Run 'drip config init' to save server and token
  Subsequent: Just run 'drip http <port>'

Note: Uses TCP over TLS 1.3 for secure communication`,
	Args: cobra.ExactArgs(1),
	RunE: runHTTP,
}

func init() {
	httpCmd.Flags().StringVarP(&subdomain, "subdomain", "n", "", "Custom subdomain (optional)")
	httpCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "Run in background (daemon mode)")
	httpCmd.Flags().StringVarP(&localAddress, "address", "a", "127.0.0.1", "Local address to forward to (default: 127.0.0.1)")
	httpCmd.Flags().BoolVar(&daemonMarker, "daemon-child", false, "Internal flag for daemon child process")
	httpCmd.Flags().MarkHidden("daemon-child")
	rootCmd.AddCommand(httpCmd)
}

func runHTTP(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	if daemonMode && !daemonMarker {
		daemonArgs := append([]string{"http"}, args...)
		daemonArgs = append(daemonArgs, "--daemon-child")
		if subdomain != "" {
			daemonArgs = append(daemonArgs, "--subdomain", subdomain)
		}
		if localAddress != "127.0.0.1" {
			daemonArgs = append(daemonArgs, "--address", localAddress)
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
		return StartDaemon("http", port, daemonArgs)
	}

	if err := utils.InitLogger(verbose); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer utils.Sync()

	logger := utils.GetLogger()

	var serverAddr, token string

	if serverURL == "" {
		cfg, err := config.LoadClientConfig("")
		if err != nil {
			return fmt.Errorf(`configuration not found.

Please run 'drip config init' first, or use flags:
  drip http %d --server SERVER:PORT --token TOKEN`, port)
		}
		serverAddr = cfg.Server
		token = cfg.Token
	} else {
		serverAddr = serverURL
		token = authToken
	}

	if serverAddr == "" {
		return fmt.Errorf("server address is required")
	}

	connConfig := &tcp.ConnectorConfig{
		ServerAddr: serverAddr,
		Token:      token,
		TunnelType: protocol.TunnelTypeHTTP,
		LocalHost:  localAddress,
		LocalPort:  port,
		Subdomain:  subdomain,
		Insecure:   insecure,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	reconnectAttempts := 0
	for {
		connector := tcp.NewConnector(connConfig, logger)

		if reconnectAttempts == 0 {
			fmt.Printf("\033[36m🔌 Connecting to %s...\033[0m\n", serverAddr)
		} else {
			fmt.Printf("\033[33m🔄 Reconnecting to %s (attempt %d/%d)...\033[0m\n", serverAddr, reconnectAttempts, maxReconnectAttempts)
		}

		if err := connector.Connect(); err != nil {
			if isNonRetryableError(err) {
				return fmt.Errorf("failed to connect: %w", err)
			}

			reconnectAttempts++
			if reconnectAttempts >= maxReconnectAttempts {
				return fmt.Errorf("failed to connect after %d attempts: %w", maxReconnectAttempts, err)
			}
			fmt.Printf("\033[31m✗ Connection failed: %v\033[0m\n", err)
			fmt.Printf("\033[90m  Retrying in %v...\033[0m\n", reconnectInterval)

			select {
			case <-quit:
				fmt.Println("\n\033[33m🛑 Shutting down...\033[0m")
				return nil
			case <-time.After(reconnectInterval):
				continue
			}
		}

		reconnectAttempts = 0

		if daemonMarker {
			daemonInfo := &DaemonInfo{
				PID:        os.Getpid(),
				Type:       "http",
				Port:       port,
				Subdomain:  subdomain,
				Server:     serverAddr,
				URL:        connector.GetURL(),
				StartTime:  time.Now(),
				Executable: os.Args[0],
			}
			if err := SaveDaemonInfo(daemonInfo); err != nil {
				logger.Warn("Failed to save daemon info", zap.Error(err))
			}
		}

		fmt.Println()
		fmt.Println("\033[1;32m╔══════════════════════════════════════════════════════════════════╗\033[0m")
		fmt.Println("\033[1;32m║\033[0m              \033[1;37m🚀 HTTP Tunnel Connected Successfully!\033[0m              \033[1;32m║\033[0m")
		fmt.Println("\033[1;32m╠══════════════════════════════════════════════════════════════════╣\033[0m")
		fmt.Printf("\033[1;32m║\033[0m  \033[1;37mTunnel URL:\033[0m                                                   \033[1;32m║\033[0m\n")
		fmt.Printf("\033[1;32m║\033[0m  \033[1;36m%-60s\033[0m \033[1;32m║\033[0m\n", connector.GetURL())
		fmt.Println("\033[1;32m║\033[0m                                                                  \033[1;32m║\033[0m")
		displayAddr := localAddress
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

		latencyCh := make(chan time.Duration, 1)
		connector.SetLatencyCallback(func(latency time.Duration) {
			select {
			case latencyCh <- latency:
			default:
			}
		})

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
					stats := connector.GetStats()
					if stats != nil {
						stats.UpdateSpeed()
						snapshot := stats.GetSnapshot()

						fmt.Print("\033[8A")

						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mLatency:\033[0m    %s%-40s\033[1;32m║\033[0m\n", formatLatency(lastLatency), "")

						trafficStr := fmt.Sprintf("↓ %s  ↑ %s", tcp.FormatBytes(snapshot.TotalBytesIn), tcp.FormatBytes(snapshot.TotalBytesOut))
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mTraffic:\033[0m    \033[36m%-48s\033[0m\033[1;32m║\033[0m\n", trafficStr)

						speedStr := fmt.Sprintf("↓ %s  ↑ %s", tcp.FormatSpeed(snapshot.SpeedIn), tcp.FormatSpeed(snapshot.SpeedOut))
						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mSpeed:\033[0m      \033[33m%-48s\033[0m\033[1;32m║\033[0m\n", speedStr)

						fmt.Printf("\r\033[1;32m║\033[0m  \033[90mRequests:\033[0m   \033[35m%-47d\033[0m\033[1;32m║\033[0m\n", snapshot.TotalRequests)

						fmt.Print("\033[4B")
					}
				case <-stopDisplay:
					return
				}
			}
		}()

		go func() {
			connector.Wait()
			close(disconnected)
		}()

		select {
		case <-quit:
			close(stopDisplay)
			fmt.Println("\n\n\033[33m🛑 Shutting down...\033[0m")
			connector.Close()
			if daemonMarker {
				RemoveDaemonInfo("http", port)
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

func formatLatency(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 50 {
		return fmt.Sprintf("\033[32m%dms\033[0m", ms)
	} else if ms < 100 {
		return fmt.Sprintf("\033[33m%dms\033[0m", ms)
	} else if ms < 200 {
		return fmt.Sprintf("\033[38;5;208m%dms\033[0m", ms)
	}
	return fmt.Sprintf("\033[31m%dms\033[0m", ms)
}

func isNonRetryableError(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "subdomain is already taken") ||
		strings.Contains(errStr, "subdomain is reserved") ||
		strings.Contains(errStr, "invalid subdomain") {
		return true
	}
	if strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "Invalid authentication token") {
		return true
	}
	return false
}
