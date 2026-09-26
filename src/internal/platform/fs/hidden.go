package fs

import (
	"path/filepath"
	"sync"
)

// HiddenChecker reports whether entries under root are hidden by the host platform's convention,
// memoizing per-directory verdicts so a tree walk pays at most one lookup per directory rather
// than one per file (relevant on Windows, where hidden is a file attribute set on any ancestor).
// Safe for concurrent use.
type HiddenChecker struct {
	root string

	mu   sync.Mutex
	dirs map[string]bool // relative dir -> hidden (itself or an ancestor)
}

// NewHiddenChecker returns a checker for entries under root.
func NewHiddenChecker(root string) *HiddenChecker {
	return &HiddenChecker{root: root, dirs: make(map[string]bool)}
}

// IsHidden reports whether relPath is hidden: its final component is hidden, or any parent
// directory is. relPath is a clean relative path (filepath.Rel output or a directory entry
// name); root itself is never evaluated, so a hidden root does not hide its whole tree.
// A nil receiver is safe and reports false, mirroring exclude.Matcher.
func (h *HiddenChecker) IsHidden(relPath string) bool {
	if h == nil {
		return false
	}

	if h.dirHidden(filepath.Dir(relPath)) {
		return true
	}

	return hiddenComponent(filepath.Join(h.root, relPath))
}

func (h *HiddenChecker) dirHidden(relDir string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.dirHiddenLocked(relDir)
}

// dirHiddenLocked reports whether relDir or any ancestor is hidden. relDir is always clean
// (filepath.Dir output), so it is keyed as-is.
func (h *HiddenChecker) dirHiddenLocked(relDir string) bool {
	if relDir == "." {
		return false
	}

	if hidden, ok := h.dirs[relDir]; ok {
		return hidden
	}

	hidden := h.dirHiddenLocked(filepath.Dir(relDir)) || hiddenComponent(filepath.Join(h.root, relDir))
	h.dirs[relDir] = hidden

	return hidden
}
