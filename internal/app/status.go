package app

import tea "charm.land/bubbletea/v2"

// Gloss reports what it is doing through the Program Status Protocol
// (OSC 7501), which Bubble Tea sends from the view: only changes reach the
// terminal, so redrawing a frame or a GIF costs nothing.
//
// Gloss owns the root record while it runs in the foreground, as the program
// in the terminal. It never clears it, because a root clear removes every
// record in the terminal, its parent's included: it leaves idle, done or
// error behind when it exits. Messages are generic: never a path, an address
// or anything a sender wrote.
const statusApp = "gloss"

func report(state tea.ProgramState, message string) *tea.ProgramStatus {
	return &tea.ProgramStatus{State: state, App: statusApp, Message: message}
}

// programStatus is the viewer's status.
func (m *Model) programStatus() *tea.ProgramStatus {
	switch {
	case m.exiting && m.picked != nil:
		return report(tea.ProgramStateDone, "Files chosen")
	case m.exiting:
		return report(tea.ProgramStateIdle, "")
	case m.opts.Pick:
		// Choosing files for a caller is the person's task, whatever the
		// viewer is doing meanwhile.
		return report(tea.ProgramStateBlocked, "Waiting for files")
	case m.loading:
		return report(tea.ProgramStateWorking, "Loading")
	case m.search != nil && m.search.running:
		return report(tea.ProgramStateWorking, "Searching")
	case m.err != nil && m.screen == screenDocument:
		return report(tea.ProgramStateError, "Could not open the file")
	}
	return report(tea.ProgramStateIdle, "")
}
