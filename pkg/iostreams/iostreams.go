// Package iostreams exposes process IO handles and terminal metadata.
package iostreams

import (
	"io"
	"os"
	"sync"

	"golang.org/x/term"
)

// IOStreams collects input and output streams for command execution.
type IOStreams struct {
	In     io.ReadCloser
	Out    io.Writer
	ErrOut io.Writer

	isStdinTTY  bool
	isStdoutTTY bool
	isStderrTTY bool

	colorEnabled bool
	once         sync.Once
}

// System returns IOStreams bound to the current process standard streams.
func System() *IOStreams {
	isTTY := func(f *os.File) bool {
		if f == nil {
			return false
		}
		return term.IsTerminal(int(f.Fd()))
	}
	return &IOStreams{
		In:          os.Stdin,
		Out:         os.Stdout,
		ErrOut:      os.Stderr,
		isStdinTTY:  isTTY(os.Stdin),
		isStdoutTTY: isTTY(os.Stdout),
		isStderrTTY: isTTY(os.Stderr),
	}
}

// CanPrompt reports whether stdin is a TTY suitable for interactive prompts.
func (s *IOStreams) CanPrompt() bool { return s != nil && s.isStdinTTY }

// IsStdoutTTY reports whether stdout is attached to a terminal.
func (s *IOStreams) IsStdoutTTY() bool { return s != nil && s.isStdoutTTY }

// ColorEnabled returns true when ANSI colour output should be rendered.
func (s *IOStreams) ColorEnabled() bool {
	if s == nil {
		return false
	}
	s.once.Do(func() { s.colorEnabled = s.isStdoutTTY })
	return s.colorEnabled
}
