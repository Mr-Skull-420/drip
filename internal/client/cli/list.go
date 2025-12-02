package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	interactiveMode bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all running background tunnels",
	Long: `List all running background tunnels.

Example:
  drip list                     Show all running tunnels
  drip list -i                  Interactive mode (select to attach/stop)

This command shows:
  - Tunnel type (HTTP/TCP)
  - Local port being tunneled
  - Public URL
  - Process ID (PID)
  - Uptime

In interactive mode, you can select a tunnel to:
  - Attach: View real-time logs
  - Stop: Terminate the tunnel`,
	Aliases: []string{"ls", "ps", "status"},
	RunE:    runList,
}

func init() {
	listCmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Interactive mode for attach/stop")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Print header
	fmt.Println()
	fmt.Println("\033[1;37mRunning Tunnels\033[0m")
	fmt.Println("\033[90m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Printf("\033[1m%-4s  %-6s  %-6s  %-40s  %-8s  %s\033[0m\n", "#", "TYPE", "PORT", "URL", "PID", "UPTIME")
	fmt.Println("\033[90m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")

	idx := 1
	for _, d := range daemons {
		// Check if process is still running
		if !IsProcessRunning(d.PID) {
			// Clean up stale entry
			RemoveDaemonInfo(d.Type, d.Port)
			continue
		}

		// Calculate uptime
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
		if len(url) > 40 {
			url = url[:37] + "..."
		}

		fmt.Printf("\033[1;36m%-4d\033[0m  %-15s  %-6d  %-40s  %-8d  %s\n",
			idx, typeStr, d.Port, url, d.PID, FormatDuration(uptime))
		idx++
	}

	fmt.Println("\033[90m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println()

	// Interactive mode or show commands
	if interactiveMode || shouldPromptForAction() {
		return runInteractiveList(daemons)
	}

	fmt.Println("Commands:")
	fmt.Println("  \033[36mdrip list -i\033[0m               Interactive mode")
	fmt.Println("  \033[36mdrip attach http 3000\033[0m      Attach to tunnel (view logs)")
	fmt.Println("  \033[36mdrip stop http 3000\033[0m        Stop tunnel")
	fmt.Println("  \033[36mdrip stop all\033[0m              Stop all tunnels")

	return nil
}

func shouldPromptForAction() bool {
	// Check if running in a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	// Always prompt when there are tunnels running
	return true
}

func runInteractiveList(daemons []*DaemonInfo) error {
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
		return nil
	}

	// Prompt for action
	fmt.Print("Select a tunnel (number) or 'q' to quit: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" || input == "q" || input == "Q" {
		return nil
	}

	// Parse selection
	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(runningDaemons) {
		return fmt.Errorf("invalid selection: %s", input)
	}

	selectedDaemon := runningDaemons[selection-1]

	// Prompt for action
	fmt.Println()
	fmt.Printf("Selected: \033[1m%s\033[0m tunnel on port \033[1m%d\033[0m\n", strings.ToUpper(selectedDaemon.Type), selectedDaemon.Port)
	fmt.Println()
	fmt.Println("What would you like to do?")
	fmt.Println("  \033[36m1.\033[0m Attach (view logs)")
	fmt.Println("  \033[36m2.\033[0m Stop tunnel")
	fmt.Println("  \033[90mq. Cancel\033[0m")
	fmt.Println()
	fmt.Print("Choose an action: ")

	actionInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	actionInput = strings.TrimSpace(actionInput)
	switch actionInput {
	case "1":
		// Attach to daemon
		return attachToDaemon(selectedDaemon)
	case "2":
		// Stop daemon
		return stopDaemon(selectedDaemon.Type, selectedDaemon.Port)
	case "q", "Q", "":
		return nil
	default:
		return fmt.Errorf("invalid action: %s", actionInput)
	}
}
