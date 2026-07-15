package app

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Quit   key.Binding
	Help   key.Binding
	Tab    key.Binding
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Kill   key.Binding
	Export key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Escape: key.NewBinding(key.WithKeys("escape"), key.WithHelp("esc", "back")),
		Kill:   key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "kill")),
		Export: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Help, k.Tab, k.Enter, k.Kill}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Escape},
		{k.Tab, k.Kill, k.Export},
		{k.Help, k.Quit},
	}
}
