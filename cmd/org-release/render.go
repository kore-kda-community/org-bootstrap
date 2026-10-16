package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

var stdin = bufio.NewReader(os.Stdin)

// ANSI colors — disabled if NO_COLOR is set.
var (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cRed    = "\033[31m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cMag    = "\033[35m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		cReset, cBold, cDim, cGreen, cRed, cYellow, cCyan, cMag = "", "", "", "", "", "", "", ""
	}
}

func hdr(format string, args ...any) {
	fmt.Printf("\n%s%s── %s ──%s\n", cBold, cCyan, fmt.Sprintf(format, args...), cReset)
}

func ok(format string, args ...any) {
	fmt.Printf("  %s✓%s %s\n", cGreen, cReset, fmt.Sprintf(format, args...))
}

func warn(format string, args ...any) {
	fmt.Printf("  %s!%s %s\n", cYellow, cReset, fmt.Sprintf(format, args...))
}

func errf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s✗%s %s\n", cRed, cReset, fmt.Sprintf(format, args...))
}

// promptLine reads a line of input with a default.
func promptLine(question, def string) string {
	if def != "" {
		fmt.Printf("  %s [%s]: ", question, def)
	} else {
		fmt.Printf("  %s: ", question)
	}
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// promptYN asks a yes/no question with a default.
func promptYN(question string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fmt.Printf("  %s [%s]: ", question, hint)
	line, _ := stdin.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return strings.HasPrefix(line, "y")
}

// pause waits for Enter.
func pause() {
	fmt.Printf("  %s(press enter)%s ", cDim, cReset)
	stdin.ReadString('\n')
}

// humanAge renders a duration since t as "3s", "5m", "2h", "4d", "3w",
// "2mo", "1y" — same vocabulary as release.sh.
func humanAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	secs := int(time.Since(t).Seconds())
	if secs < 0 {
		secs = 0
	}
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh", secs/3600)
	case secs < 604800:
		return fmt.Sprintf("%dd", secs/86400)
	case secs < 2592000:
		return fmt.Sprintf("%dw", secs/604800)
	case secs < 31536000:
		return fmt.Sprintf("%dmo", secs/2592000)
	default:
		return fmt.Sprintf("%dy", secs/31536000)
	}
}

// truncate shortens s to width, adding an ellipsis if cut.
func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

// promptSecret reads a line of input with terminal echo disabled. Falls
// back to a plain read when stdin isn't a terminal (piped input for tests).
func promptSecret(question string) string {
	fmt.Printf("  %s: ", question)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	// not a terminal — read plainly so pipes still work
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}
