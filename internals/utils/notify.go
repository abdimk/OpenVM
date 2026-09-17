package utils

import "sync/atomic"

type FooterHint struct {
	Text  string
	Color string
	Bg    string
}

var footerHint atomic.Value

func EmitFooterHint(text string) {
	footerHint.Store(FooterHint{Text: text})
}

func EmitFooterHintColored(text, color string) {
	footerHint.Store(FooterHint{Text: text, Color: color})
}

func EmitFooterHintHighlighted(text, color, bg string) {
	footerHint.Store(FooterHint{Text: text, Color: color, Bg: bg})
}

func CurrentFooterHint() FooterHint {
	if v, ok := footerHint.Load().(FooterHint); ok {
		return v
	}
	return FooterHint{}
}

func ClearFooterHint() {
	footerHint.Store(FooterHint{})
}
