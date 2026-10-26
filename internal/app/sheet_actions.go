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
	entries = append(entries, m.chartEntries()...)
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

// chartEntries offer the chart of the sheet's visible columns: showing or
// hiding it, and, once drawn, opening it in a browser where the host can.
func (m *Model) chartEntries() []menuEntry {
	id, label := "chart", i18n.T("app.actions.chart", "Chart these columns")
	if m.chartPane != nil {
		id, label = "hide-chart", i18n.T("app.actions.hideChart", "Hide the chart")
	}
	entries := []menuEntry{{id, label, func(m *Model) tea.Cmd { return m.toggleChartPane() }}}
	if p := m.chartPane; p != nil && p.shown() && m.opts.OpenWith != nil && m.opts.Scratch != nil {
		entries = append(entries, menuEntry{"chart-browser", i18n.T("app.actions.chartBrowser", "Open chart in browser"), func(m *Model) tea.Cmd { return m.openChartPage() }})
	}
	return entries
}
