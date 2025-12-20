package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Output format constants
const (
	OutputPretty = "pretty"
	OutputJSON   = "json"
	OutputYAML   = "yaml"
)

// Unicode icons
const (
	IconSuccess  = "✓"
	IconFailure  = "✗"
	IconWarning  = "⚠"
	IconSkipped  = "○"
	IconInfo     = "●"
)

// Spinner frames for animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StepStatus represents the status of a step
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepSuccess
	StepFailure
	StepSkipped
	StepWarning
)

// Step represents a single operation in the progress display
type Step struct {
	Name    string
	Status  StepStatus
	Message string
	Details string
}

// ProgressDisplay manages the interactive CLI output
type ProgressDisplay struct {
	steps        []*Step
	mu           sync.Mutex
	stopSpinner  chan struct{}
	spinnerDone  chan struct{}
	currentFrame int
	enabled      bool
	interactive  bool
	outputFormat string
	rendered     int // number of lines currently rendered
}

// NewProgressDisplay creates a new progress display
func NewProgressDisplay(format string) *ProgressDisplay {
	isPretty := format == OutputPretty
	isTTY := isTerminal()
	return &ProgressDisplay{
		steps:        make([]*Step, 0),
		stopSpinner:  make(chan struct{}),
		spinnerDone:  make(chan struct{}),
		enabled:      isPretty,
		interactive:  isPretty && isTTY,
		outputFormat: format,
	}
}

// AddStep adds a new step to the display
func (p *ProgressDisplay) AddStep(name string) *Step {
	p.mu.Lock()
	defer p.mu.Unlock()

	step := &Step{
		Name:   name,
		Status: StepPending,
	}
	p.steps = append(p.steps, step)
	return step
}

// StartStep marks a step as running and displays it
func (p *ProgressDisplay) StartStep(step *Step, message string) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	step.Status = StepRunning
	step.Message = message
	p.mu.Unlock()

	p.render()
}

// CompleteStep marks a step as completed
func (p *ProgressDisplay) CompleteStep(step *Step, status StepStatus, message string, details string) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	step.Status = status
	step.Message = message
	step.Details = details
	p.mu.Unlock()

	p.render()
}

// Start begins the spinner animation
func (p *ProgressDisplay) Start() {
	if !p.enabled {
		return
	}

	if p.interactive {
		// Hide cursor for interactive mode
		fmt.Print("\033[?25l")

		go func() {
			ticker := time.NewTicker(80 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-p.stopSpinner:
					close(p.spinnerDone)
					return
				case <-ticker.C:
					p.mu.Lock()
					p.currentFrame = (p.currentFrame + 1) % len(spinnerFrames)
					p.mu.Unlock()
					p.render()
				}
			}
		}()
	} else {
		close(p.spinnerDone)
	}
}

// Stop stops the spinner animation
func (p *ProgressDisplay) Stop() {
	if !p.enabled {
		return
	}

	if p.interactive {
		close(p.stopSpinner)
		<-p.spinnerDone

		// Show cursor
		fmt.Print("\033[?25h")
	}

	// Final render
	p.renderFinal()
	fmt.Println()
}

// render displays all steps (interactive mode with cursor movement)
func (p *ProgressDisplay) render() {
	if !p.interactive {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Move cursor up to the start of our output
	if p.rendered > 0 {
		fmt.Printf("\033[%dA", p.rendered)
	}

	for _, step := range p.steps {
		// Clear line
		fmt.Print("\033[2K\r")

		icon := p.getIcon(step.Status)
		color := p.getColor(step.Status)

		if step.Status == StepRunning {
			icon = spinnerFrames[p.currentFrame]
		}

		// Print step with color
		fmt.Printf("%s%s%s %s", color, icon, "\033[0m", step.Name)

		if step.Message != "" {
			fmt.Printf(" %s%s%s", "\033[90m", step.Message, "\033[0m")
		}

		if step.Details != "" && step.Status != StepRunning {
			fmt.Printf(" %s[%s]%s", "\033[90m", step.Details, "\033[0m")
		}

		fmt.Println()
	}

	p.rendered = len(p.steps)
}

// renderFinal displays all steps in their final state
func (p *ProgressDisplay) renderFinal() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.interactive {
		// Move cursor up to the start of our output
		if p.rendered > 0 {
			fmt.Printf("\033[%dA", p.rendered)
		}
	}

	for _, step := range p.steps {
		if p.interactive {
			// Clear line
			fmt.Print("\033[2K\r")
		}

		icon := p.getIcon(step.Status)
		color := p.getColor(step.Status)

		// Print step with color
		fmt.Printf("%s%s%s %s", color, icon, "\033[0m", step.Name)

		if step.Message != "" {
			fmt.Printf(" %s%s%s", "\033[90m", step.Message, "\033[0m")
		}

		if step.Details != "" {
			fmt.Printf(" %s[%s]%s", "\033[90m", step.Details, "\033[0m")
		}

		fmt.Println()
	}
}

// getIcon returns the icon for a status
func (p *ProgressDisplay) getIcon(status StepStatus) string {
	switch status {
	case StepPending:
		return "○"
	case StepRunning:
		return spinnerFrames[p.currentFrame]
	case StepSuccess:
		return IconSuccess
	case StepFailure:
		return IconFailure
	case StepSkipped:
		return IconSkipped
	case StepWarning:
		return IconWarning
	default:
		return "?"
	}
}

// getColor returns ANSI color code for a status
func (p *ProgressDisplay) getColor(status StepStatus) string {
	switch status {
	case StepPending:
		return "\033[90m" // Gray
	case StepRunning:
		return "\033[36m" // Cyan
	case StepSuccess:
		return "\033[32m" // Green
	case StepFailure:
		return "\033[31m" // Red
	case StepSkipped:
		return "\033[33m" // Yellow
	case StepWarning:
		return "\033[33m" // Yellow
	default:
		return "\033[0m"
	}
}

// PrintHeader prints a header line
func (p *ProgressDisplay) PrintHeader(text string) {
	if !p.enabled {
		return
	}
	fmt.Printf("\033[1m%s\033[0m\n", text)
}

// PrintInfo prints an info line
func (p *ProgressDisplay) PrintInfo(label, value string) {
	if !p.enabled {
		return
	}
	fmt.Printf("  %s%s:%s %s\n", "\033[90m", label, "\033[0m", value)
}

// PrintSeparator prints a separator line
func (p *ProgressDisplay) PrintSeparator() {
	if !p.enabled {
		return
	}
	fmt.Println()
}

// FormatOutput formats the final output based on the format type
func FormatOutput(output Output, format string) (string, error) {
	switch format {
	case OutputJSON:
		jsonBytes, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return "", fmt.Errorf("error marshalling JSON: %w", err)
		}
		return string(jsonBytes), nil

	case OutputYAML:
		yamlBytes, err := yaml.Marshal(output)
		if err != nil {
			return "", fmt.Errorf("error marshalling YAML: %w", err)
		}
		return string(yamlBytes), nil

	case OutputPretty:
		return formatPrettySummary(output), nil

	default:
		return "", fmt.Errorf("unknown output format: %s", format)
	}
}

// formatPrettySummary creates a human-readable summary
func formatPrettySummary(output Output) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("\033[1m%s Summary\033[0m\n", IconInfo))
	sb.WriteString(fmt.Sprintf("  Host: %s\n", output.FQDN))
	sb.WriteString(fmt.Sprintf("  IPv4: %s\n", output.SourceUPs.IPV4))
	if output.SourceUPs.IPV6 != "" {
		sb.WriteString(fmt.Sprintf("  IPv6: %s\n", output.SourceUPs.IPV6))
	}
	sb.WriteString("\n")

	for _, result := range output.Results {
		sb.WriteString(fmt.Sprintf("\033[1mServer: %s\033[0m\n", result.Server))

		// A Records
		if len(result.ARecordsFound) > 0 {
			sb.WriteString(fmt.Sprintf("  A Records: %s\n", strings.Join(result.ARecordsFound, ", ")))
		} else {
			sb.WriteString("  A Records: (none)\n")
		}

		for _, update := range result.ARecordUpdates {
			icon := getStatusIcon(update.Status)
			sb.WriteString(fmt.Sprintf("    %s %s: %s\n", icon, update.IP, update.Message))
		}

		// AAAA Records
		if len(result.AAAARecordsFound) > 0 {
			sb.WriteString(fmt.Sprintf("  AAAA Records: %s\n", strings.Join(result.AAAARecordsFound, ", ")))
		}

		for _, update := range result.AAAARecordUpdates {
			icon := getStatusIcon(update.Status)
			sb.WriteString(fmt.Sprintf("    %s %s: %s\n", icon, update.IP, update.Message))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// getStatusIcon returns an icon based on status string
func getStatusIcon(status string) string {
	switch status {
	case "success", "updated":
		return "\033[32m" + IconSuccess + "\033[0m"
	case "failed", "error":
		return "\033[31m" + IconFailure + "\033[0m"
	case "skipped":
		return "\033[33m" + IconSkipped + "\033[0m"
	default:
		return "\033[90m" + IconInfo + "\033[0m"
	}
}

// PrintFinalOutput prints the output in the specified format
func PrintFinalOutput(output Output, format string) {
	formatted, err := FormatOutput(output, format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(formatted)
}
