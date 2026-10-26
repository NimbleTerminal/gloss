package app

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// cellEntries lists what can be done with the cell under the cursor in
// this host. An address adds opening it, where the host can.
func (m *Model) cellEntries() []menuEntry {
	v := m.sheet
	if v == nil || v.rows() == 0 {
		return nil
	}
	value, row, address := v.value(), v.rowText(), v.url()
	entries := []menuEntry{
		{"copy-value", i18n.T("app.actions.copyValue", "Copy value"), func(m *Model) tea.Cmd {
			m.note = i18n.T("app.note.valueCopied", "Value copied")
			return tea.SetClipboard(value)
		}},
		{"copy-row", i18n.T("app.actions.copyRow", "Copy row"), func(m *Model) tea.Cmd {
			m.note = i18n.T("app.note.rowCopied", "Row copied")
			return tea.SetClipboard(row)
		}},
	}
	if address == "" {
		return entries
	}
	entries = append(entries, fetchEntries(m, address)...)
	if m.opts.OpenLink != nil && webLink(address) != "" {
		entries = append(entries, menuEntry{"open-browser", i18n.T("app.actions.openBrowser", "Open in browser"), func(m *Model) tea.Cmd {
			open, generation := m.opts.OpenLink, m.generation
			return func() tea.Msg { return linkOpened{generation, open(address)} }
		}})
	}
	return append(entries, menuEntry{"show-qr", i18n.T("app.actions.showQR", "Show QR"), func(m *Model) tea.Cmd { return m.openQR(address) }})
}
