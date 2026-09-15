package utils

import "sync/atomic"

// FooterHint is a transient status line the active screen can publish to the
// shared footer (e.g. download state, "already installed" notices).
type FooterHint struct {
	Text  string
	Color string // optional lipgloss-compatible color; "" = default style
}

// footerHint holds the current transient hint. It is safe for concurrent use
// because download pipelines run on a goroutine while Update runs on the UI
// one.
var footerHint atomic.Value

// EmitFooterHint publishes a transient footer message in the default color.
func EmitFooterHint(text string) {
	footerHint.Store(FooterHint{Text: text})
}

// EmitFooterHintColored publishes a transient footer message with a custom
// color (e.g. "#FFA500" for orange notices).
func EmitFooterHintColored(text, color string) {
	footerHint.Store(FooterHint{Text: text, Color: color})
}

// CurrentFooterHint returns the last published footer message.
func CurrentFooterHint() FooterHint {
	if v, ok := footerHint.Load().(FooterHint); ok {
		return v
	}
	return FooterHint{}
}

// ClearFooterHint removes the transient footer message.
func ClearFooterHint() {
	footerHint.Store(FooterHint{})
}
