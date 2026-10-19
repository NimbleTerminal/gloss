package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	ntpicture "nimbleterminal.dev/ntpicture"
	"nimbleterminal.dev/ntqrcode/qrcode"
)

// Handoff displays the already-running web picker's URL. The CLI owns the
// server, its lifetime, and results; this screen owns only terminal graphics.
type Handoff struct {
	terminal        *ntpicture.TerminalSession
	kitty           bool
	qr              *qrcode.Model
	address, prompt string
	width, height   int
	autoKitty       bool
	closing         bool
	finished        bool
	end             HandoffEnd
	done            <-chan HandoffEnd
	cancel          func()
}

// HandoffEnd is how the request the screen shows was settled.
type HandoffEnd int

const (
	HandoffReceived HandoffEnd = iota + 1 // Files were sent.
	HandoffDeclined                       // The request was cancelled, here or on the device.
	HandoffTimedOut
	HandoffFailed
)

type handoffFinished struct{ end HandoffEnd }

// NewHandoff shows address until the request is settled; the caller sends
// how on done.
func NewHandoff(address, prompt, render string, done <-chan HandoffEnd, cancel func()) (*Handoff, error) {
	code, err := qrcode.Encode(address, qrcode.Options{})
	if err != nil {
		return nil, err
	}
	terminal := newTerminalSession(render)
	qr, err := qrcode.New(code, qrcode.Config{Session: terminal, NextID: ntpicture.NextImageID})
	if err != nil {
		terminal.Close()
		return nil, err
	}
	return &Handoff{terminal: terminal, kitty: terminal.Snapshot().Kitty == ntpicture.KittyCapabilitySupported, qr: qr, address: address, prompt: prompt, autoKitty: render != "glyph", done: done, cancel: cancel}, nil
}

func (m *Handoff) Init() tea.Cmd {
	return tea.Batch(m.terminal.Init(), func() tea.Msg {
		return handoffFinished{<-m.done}
	})
}

func (m *Handoff) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.finished {
		return m, nil
	}
	commands := []tea.Cmd{m.terminal.Update(msg)}
	state := m.terminal.Snapshot()
	switch v := msg.(type) {
	case handoffFinished:
		m.finished, m.end = true, v.end
		return m, tea.Sequence(m.qr.Close(), tea.Quit)
	case tea.WindowSizeMsg:
		m.width, m.height = max(0, v.Width), max(0, v.Height)
	case tea.KeyPressMsg:
		switch v.String() {
		case "q", "esc", "ctrl+c":
			if !m.closing {
				m.closing = true
				return m, func() tea.Msg { m.cancel(); return nil }
			}
		case "g":
			m.autoKitty = false
			m.kitty = !m.kitty && state.Kitty == ntpicture.KittyCapabilitySupported
		}
	}
	if m.autoKitty && state.Kitty == ntpicture.KittyCapabilitySupported {
		m.kitty = true
	}
	commands = append(commands, m.qr.SetTerminal(m.kitty, state.CellPixelWidth, state.CellPixelHeight), m.qr.SetSize(m.width, max(0, m.height-m.textRows())), m.qr.Update(msg))
	return m, tea.Batch(commands...)
}

func (m *Handoff) textRows() int {
	if m.prompt != "" {
		return 4
	}
	return 3
}

func (m *Handoff) View() tea.View {
	var lines []string
	line := func(text string) string { return ansi.Truncate(safe(text), max(1, m.width), "…") }
	if m.height <= m.textRows() {
		lines = []string{line("Enlarge terminal to show the QR")}
	} else {
		lines = append(lines, line("Scan to send files and a message"))
		if m.prompt != "" {
			lines = append(lines, line(strings.Join(strings.Fields(m.prompt), " ")))
		}
		if err := m.qr.Err(); err != nil {
			lines = append(lines, line(err.Error()+"; enlarge terminal"))
		} else {
			// Never truncate or reflow the code or its quiet zone.
			lines = append(lines, m.qr.View())
		}
		lines = append(lines, line(m.address))
		if m.closing {
			lines = append(lines, line("Closing request…"))
		} else {
			lines = append(lines, line("Waiting for Send on your device · g graphics · q/Esc cancel"))
		}
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.ProgramStatus = m.programStatus()
	return v
}

// programStatus is the handoff's status: blocked on the person until the
// request settles, then what became of it.
func (m *Handoff) programStatus() *tea.ProgramStatus {
	switch {
	case m.end == HandoffReceived:
		return report(tea.ProgramStateDone, "Files received")
	case m.end == HandoffTimedOut:
		return report(tea.ProgramStateError, "Timed out waiting for files")
	case m.end == HandoffFailed:
		return report(tea.ProgramStateError, "Could not receive files")
	case m.end == HandoffDeclined, m.closing:
		return report(tea.ProgramStateIdle, "")
	}
	return report(tea.ProgramStateBlocked, "Waiting for files")
}

// Close releases discovery resources after the terminal event loop exits.
func (m *Handoff) Close() error {
	m.qr.Close()
	m.terminal.Close()
	return nil
}
