// Package tui provides simple interactive prompts on stdin/stdout.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var reader = bufio.NewReader(os.Stdin)

// PromptString asks for a line of input. Returns the default if the user
// presses Enter without typing anything.
func PromptString(question, def string) (string, error) {
	if def != "" {
		fmt.Printf("  %s [%s]: ", question, def)
	} else {
		fmt.Printf("  %s: ", question)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

// PromptSecret asks for sensitive input. On a terminal, characters are
// hidden via x/term. When stdin is piped, falls back to a plain read so
// scripted flows still work.
func PromptSecret(question string) (string, error) {
	fmt.Printf("  %s: ", question)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// PromptYN asks a yes/no question, returning the default on empty input.
func PromptYN(question string, def bool) (bool, error) {
	suffix := "[y/N]"
	if def {
		suffix = "[Y/n]"
	}
	fmt.Printf("  %s %s: ", question, suffix)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def, nil
	}
	return strings.HasPrefix(line, "y"), nil
}

// PromptChoice offers "a" / "s" / "q" style picks and returns the chosen key.
func PromptChoice(question, def string, valid ...string) (string, error) {
	fmt.Printf("  %s [%s]: ", question, def)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = def
	}
	for _, v := range valid {
		if strings.EqualFold(line, v) {
			return strings.ToLower(v), nil
		}
	}
	return "", fmt.Errorf("invalid choice %q", line)
}

// ParseNumericPick parses "1 3 5" into []int{1,3,5}. Empty returns empty.
func ParseNumericPick(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	// Split on any non-digit boundary. Anything that isn't a number is
	// silently dropped — users paste loose text all the time.
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r < '0' || r > '9'
	})
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// PickByNumber prints items and prompts for "1 3 5"-style selection.
// Returns the picked items in the order the user typed them.
func PickByNumber(header string, items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	fmt.Println(header)
	for i, item := range items {
		fmt.Printf("    [%2d] %s\n", i+1, item)
	}
	fmt.Println()
	line, err := PromptString("  Selection (e.g. 1 3 5)", "")
	if err != nil {
		return nil, err
	}
	nums, err := ParseNumericPick(line)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(nums))
	for _, n := range nums {
		if n >= 1 && n <= len(items) {
			out = append(out, items[n-1])
		}
	}
	return out, nil
}

// PickOrgs prompts for org selection in one shot. Accepts:
//
//	<empty> or "a"      → every item
//	"q"                 → quit
//	"1 3 5" or "1,3,5"  → those items, in the order typed
//
// Invalid input re-prompts rather than aborting.
func PickOrgs(header string, items []string) (picked []string, quit bool, err error) {
	if len(items) == 0 {
		return nil, false, nil
	}
	fmt.Println(header)
	for i, item := range items {
		fmt.Printf("    [%2d] %s\n", i+1, item)
	}
	fmt.Println()
	for {
		fmt.Print("  Select (numbers, 'a'=all, 'q'=quit) [a]: ")
		line, rerr := reader.ReadString('\n')
		if rerr != nil {
			return nil, false, rerr
		}
		line = strings.ToLower(strings.TrimSpace(line))

		switch line {
		case "", "a":
			return items, false, nil
		case "q":
			return nil, true, nil
		}

		fields := strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		})
		nums := make([]int, 0, len(fields))
		bad := false
		for _, f := range fields {
			n, aerr := strconv.Atoi(f)
			if aerr != nil || n < 1 || n > len(items) {
				fmt.Printf("  ! %q is not a valid pick (expected 1..%d)\n", f, len(items))
				bad = true
				break
			}
			nums = append(nums, n)
		}
		if bad {
			continue
		}
		out := make([]string, 0, len(nums))
		for _, n := range nums {
			out = append(out, items[n-1])
		}
		return out, false, nil
	}
}
