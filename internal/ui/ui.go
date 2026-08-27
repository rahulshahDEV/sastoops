package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Dim    = "\033[2m"
)

func color(c, s string) string {
	if noColor() {
		return s
	}
	return c + s + Reset
}

func noColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

func BoldS(s string) string   { return color(Bold, s) }
func GreenS(s string) string  { return color(Green, s) }
func RedS(s string) string    { return color(Red, s) }
func YellowS(s string) string { return color(Yellow, s) }
func CyanS(s string) string   { return color(Cyan, s) }
func BlueS(s string) string   { return color(Blue, s) }
func DimS(s string) string    { return color(Dim, s) }

func Section(title string, args ...any) {
	if len(args) > 0 {
		title = fmt.Sprintf(title, args...)
	}
	fmt.Printf("\n%s\n", BoldS(title))
}

func Ok(format string, a ...any) {
	fmt.Printf("%s %s\n", GreenS("✔"), fmt.Sprintf(format, a...))
}

func Info(format string, a ...any) {
	fmt.Printf("%s %s\n", BlueS("ℹ"), fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Printf("%s %s\n", YellowS("⚠"), fmt.Sprintf(format, a...))
}

func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", RedS("✘"), fmt.Sprintf(format, a...))
}

func Step(format string, a ...any) {
	fmt.Printf("%s %s\n", CyanS("→"), fmt.Sprintf(format, a...))
}

type Table struct {
	header []string
	rows   [][]string
}

func NewTable(header ...string) *Table {
	return &Table{header: header}
}

func (t *Table) Add(row ...string) {
	t.rows = append(t.rows, row)
}

func (t *Table) Render() {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	hdrs := make([]string, len(t.header))
	for i, h := range t.header {
		hdrs[i] = BoldS(strings.ToUpper(h))
	}
	fmt.Fprintln(w, strings.Join(hdrs, "\t"))
	for _, r := range t.rows {
		cells := make([]string, len(r))
		for i, c := range r {
			if c == "" && i < len(t.header) {
				c = "—"
			}
			cells[i] = c
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
	w.Flush()
}

// KV renders key/value pairs aligned.
func KV(pairs [][2]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, p := range pairs {
		fmt.Fprintf(w, "%s\t%s\n", BoldS(p[0]), p[1])
	}
	w.Flush()
}

// SortedKeys returns map keys sorted for stable output.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// PrintJSON emits machine-readable output on stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Confirm asks yes/no. autoYes bypasses the prompt.
func Confirm(question string, autoYes bool) bool {
	if autoYes {
		return true
	}
	if !IsTTY() {
		return false
	}
	fmt.Fprintf(os.Stderr, "%s %s [y/N]: ", YellowS("?"), question)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

// IsTTY reports whether stdin is an interactive terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Prompt asks for a string; returns def on empty input or non-TTY.
func Prompt(question, def string) string {
	if !IsTTY() {
		return def
	}
	suffix := ""
	if def != "" {
		suffix = fmt.Sprintf(" [%s]", def)
	}
	fmt.Fprintf(os.Stderr, "%s %s%s: ", YellowS("?"), question, suffix)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// Select shows a numbered menu; returns the chosen index or -1 if not a TTY.
func Select(title string, options []string) int {
	if !IsTTY() {
		return -1
	}
	fmt.Fprintf(os.Stderr, "\n%s\n", BoldS(title))
	for i, o := range options {
		fmt.Fprintf(os.Stderr, "  %s %s\n", CyanS(fmt.Sprintf("%2d.", i+1)), o)
	}
	fmt.Fprintf(os.Stderr, "  %s %s\n", DimS(" 0."), "cancel")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "%s choose [0-%d]: ", YellowS("?"), len(options))
		line, _ := reader.ReadString('\n')
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 0 && n <= len(options) {
			if n == 0 {
				return -1
			}
			return n - 1
		}
	}
}

// Spinner is a minimal TTY spinner; degrades to dots on non-TTY.
type Spinner struct {
	msg  string
	stop chan bool
	done chan bool
}

func StartSpinner(msg string) *Spinner {
	s := &Spinner{msg: msg, stop: make(chan bool), done: make(chan bool)}
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.stop:
				close(s.done)
				return
			default:
				fmt.Fprintf(os.Stderr, "\r\033[K%s %s", CyanS(frames[i%len(frames)]), s.msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return s
}

func (s *Spinner) Stop(final string) {
	close(s.stop)
	<-s.done
	fmt.Fprintf(os.Stderr, "\r\033[K%s %s\n", GreenS("✓"), final)
}

// HumanBytes renders byte counts.
func HumanBytes(n uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
