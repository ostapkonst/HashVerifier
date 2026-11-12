package base

// RunGate enforces a single active tab run: the tab that starts locks the gate, and the app
// disables the other tabs' action buttons until that tab releases it.
type RunGate struct {
	locked   string
	OnChange func(locked bool)
}

// NewRunGate returns an unlocked gate.
func NewRunGate() *RunGate {
	return &RunGate{}
}

// Lock marks name as the running tab and notifies OnChange. It returns false when another tab
// already holds the gate, so a second start can be ignored. Must run on the GTK thread.
func (g *RunGate) Lock(name string) bool {
	if g.locked != "" {
		return false
	}

	g.locked = name
	if g.OnChange != nil {
		g.OnChange(true)
	}

	return true
}

// Unlock releases the gate when name owns it; a non-owner or an already-unlocked gate is a
// no-op. Must run on the GTK thread.
func (g *RunGate) Unlock(name string) {
	if g.locked != name {
		return
	}

	g.locked = ""
	if g.OnChange != nil {
		g.OnChange(false)
	}
}

// IsLocked reports whether any tab currently holds the gate.
func (g *RunGate) IsLocked() bool {
	return g.locked != ""
}
