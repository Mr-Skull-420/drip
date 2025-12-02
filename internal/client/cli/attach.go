package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [type] [port]",
	Short: "Attach to a running background tunnel",
	Long: `Attach to a running background tunnel to view its logs in real-time.

Examples:
  drip attach              List running tunnels and select one
  drip attach http 3000    Attach to HTTP tunnel on port 3000
  drip attach tcp 5432     Attach to TCP tunnel on port 5432

Press Ctrl+C to detach (tunnel will continue running).`,
	Aliases: []string{"logs", "tail"},
	Args:    cobra.MaximumNArgs(2),
	RunE:    runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	// Clean up stale daemons first
	CleanupStaleDaemons()

	// Get all running daemons
	daemons, err := ListAllDaemons()
	if err != nil {
		return fmt.Errorf("failed to list daemons: %w", err)
	}

	if len(daemons) == 0 {
		fmt.Println("\033[90mNo running tunnels.\033[0m")
		fmt.Println()
		fmt.Println("Start a tunnel in background with:")
		fmt.Println("  \033[36mdrip http 3000 -d\033[0m")
		fmt.Println("  \033[36mdrip tcp 5432 -d\033[0m")
		return nil
	}

	var selectedDaemon *DaemonInfo

	// If type and port are specified, find the specific daemon
	if len(args) == 2 {
		tunnelType := args[0]
		if tunnelType != "http" && tunnelType != "tcp" {
			return fmt.Errorf("invalid tunnel type: %s (must be 'http' or 'tcp')", tunnelType)
		}

		port, err := strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port number: %s", args[1])
		}

		// Find the daemon
		for _, d := range daemons {
			if d.Type == tunnelType && d.Port == port {
				if !IsProcessRunning(d.PID) {
					RemoveDaemonInfo(d.Type, d.Port)
					return fmt.Errorf("tunnel is not running (cleaned up stale entry)")
				}
				selectedDaemon = d
				break
			}
		}

		if selectedDaemon == nil {
			return fmt.Errorf("no %s tunnel running on port %d", tunnelType, port)
		}
	} else if len(args) == 0 {
		// Interactive selection
		selectedDaemon, err = selectDaemonInteractive(daemons)
		if err != nil {
			return err
		}
		if selectedDaemon == nil {
			return nil // User cancelled
		}
	} else {
		return fmt.Errorf("usage: drip attach [type port]")
	}

	// Attach to the selected daemon
	return attachToDaemon(selectedDaemon)
}

func selectDaemonInteractive(daemons []*DaemonInfo) (*DaemonInfo, error) {
	// Print header
	fmt.Println()
	fmt.Println("\033[1;37mSelect a tunnel to attach:\033[0m")
	fmt.Println("\033[90m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")

	// Filter out non-running daemons
	var runningDaemons []*DaemonInfo
	for _, d := range daemons {
		if IsProcessRunning(d.PID) {
			runningDaemons = append(runningDaemons, d)
		} else {
			RemoveDaemonInfo(d.Type, d.Port)
		}
	}

	if len(runningDaemons) == 0 {
		fmt.Println("\033[90mNo running tunnels.\033[0m")
		return nil, nil
	}

	// Print list
	for i, d := range runningDaemons {
		uptime := time.Since(d.StartTime)

		// Format type with color
		var typeStr string
		if d.Type == "http" {
			typeStr = "\033[32mHTTP\033[0m"
		} else {
			typeStr = "\033[35mTCP\033[0m"
		}

		// Truncate URL if too long
		url := d.URL
		if len(url) > 50 {
			url = url[:47] + "..."
		}

		fmt.Printf("\033[1;36m%d.\033[0m %-15s \033[90mPort:\033[0m %-6d \033[90mURL:\033[0m %-50s \033[90mUptime:\033[0m %s\n",
			i+1, typeStr, d.Port, url, FormatDuration(uptime))
	}

	fmt.Println("\033[90m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Printf("Enter number (1-%d) or 'q' to quit: ", len(runningDaemons))

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "q" || input == "Q" {
		return nil, nil
	}

	// Parse selection
	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(runningDaemons) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return runningDaemons[selection-1], nil
}

func attachToDaemon(daemon *DaemonInfo) error {
	// Get log file path
	logPath := filepath.Join(getDaemonDir(), fmt.Sprintf("%s_%d.log", daemon.Type, daemon.Port))

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logPath)
	}

	// Print header
	fmt.Println()
	fmt.Println("\033[1;32m╔══════════════════════════════════════════════════════════════════╗\033[0m")
	fmt.Printf("\033[1;32m║\033[0m  \033[1;37mAttached to %s tunnel on port %d\033[0m", strings.ToUpper(daemon.Type), daemon.Port)
	fmt.Printf("%s\033[1;32m║\033[0m\n", strings.Repeat(" ", 32-len(daemon.Type)))
	fmt.Println("\033[1;32m╠══════════════════════════════════════════════════════════════════╣\033[0m")
	fmt.Printf("\033[1;32m║\033[0m  \033[90mURL:\033[0m      \033[36m%-52s\033[0m \033[1;32m║\033[0m\n", daemon.URL)
	uptime := time.Since(daemon.StartTime)
	fmt.Printf("\033[1;32m║\033[0m  \033[90mPID:\033[0m      \033[90m%-52d\033[0m \033[1;32m║\033[0m\n", daemon.PID)
	fmt.Printf("\033[1;32m║\033[0m  \033[90mUptime:\033[0m   \033[90m%-52s\033[0m \033[1;32m║\033[0m\n", FormatDuration(uptime))
	fmt.Printf("\033[1;32m║\033[0m  \033[90mLog:\033[0m      \033[90m%-52s\033[0m \033[1;32m║\033[0m\n", truncatePath(logPath, 52))
	fmt.Println("\033[1;32m╠══════════════════════════════════════════════════════════════════╣\033[0m")
	fmt.Println("\033[1;32m║\033[0m  \033[33mPress Ctrl+C to detach (tunnel will continue running)\033[0m     \033[1;32m║\033[0m")
	fmt.Println("\033[1;32m╚══════════════════════════════════════════════════════════════════╝\033[0m")
	fmt.Println()

	// Setup signal handler
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start tail command
	tailCmd := exec.Command("tail", "-f", logPath)
	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr

	if err := tailCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tail: %w", err)
	}

	// Wait for signal or tail to exit
	done := make(chan error, 1)
	go func() {
		done <- tailCmd.Wait()
	}()

	select {
	case <-sigCh:
		// Kill tail process
		if tailCmd.Process != nil {
			tailCmd.Process.Kill()
		}
		fmt.Println()
		fmt.Println("\033[33mDetached from tunnel (tunnel is still running)\033[0m")
		fmt.Printf("Use '\033[36mdrip attach %s %d\033[0m' to reattach\n", daemon.Type, daemon.Port)
		fmt.Printf("Use '\033[36mdrip stop %s %d\033[0m' to stop the tunnel\n", daemon.Type, daemon.Port)
		return nil
	case err := <-done:
		if err != nil {
			return fmt.Errorf("tail process exited: %w", err)
		}
		return nil
	}
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	// Try to keep filename and show ... in the middle
	filename := filepath.Base(path)
	if len(filename) >= maxLen-3 {
		return "..." + filename[len(filename)-(maxLen-3):]
	}
	dirLen := maxLen - len(filename) - 3
	return path[:dirLen] + "..." + filename
}
