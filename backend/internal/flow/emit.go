package flow

import (
	"context"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// User scope
// ---------------------------------------------------------------------------

// scope is shared by everything that runs under one request, so a session
// resolved deep in the middleware chain also redacts the events emitted by
// the middleware above it, once the request returns there.
type scope struct{ user atomic.Bool }

type scopeKey struct{}

// WithScope gives ctx a scope that MarkUser can mark later. The HTTP tap
// calls it before the rest of the chain runs.
func WithScope(ctx context.Context) context.Context {
	if _, ok := ctx.Value(scopeKey{}).(*scope); ok {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, &scope{})
}

// MarkUser records that ctx belongs to a signed-in user: every event emitted
// under it from now on, and every event the enclosing request emits after
// returning, is redacted. It marks the scope ctx already carries; with none,
// it returns a context carrying a marked one.
func MarkUser(ctx context.Context) context.Context {
	if s, ok := ctx.Value(scopeKey{}).(*scope); ok {
		s.user.Store(true)
		return ctx
	}
	return UserLinked(ctx)
}

// UserLinked returns a context whose events are always redacted, for work
// done on a user's behalf outside a request (the tracked-issue check).
func UserLinked(ctx context.Context) context.Context {
	s := &scope{}
	s.user.Store(true)
	return context.WithValue(ctx, scopeKey{}, s)
}

func isUser(ctx context.Context) bool {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	return ok && s.user.Load()
}

// ---------------------------------------------------------------------------
// Emit
// ---------------------------------------------------------------------------

var current atomic.Pointer[Publisher]

// Install makes p the process's publisher; nil uninstalls it. Until one is
// installed, Emit does nothing — cmd/migrate, cmd/repoimport and flowd never
// install one.
func Install(p *Publisher) { current.Store(p) }

// Self is the node this process is in the visualizer, or "" with no
// publisher installed.
func Self() Node {
	if p := current.Load(); p != nil {
		return p.self
	}
	return ""
}

// Active reports whether an emitted event would be published. Callers with
// work to do before building an event (parsing SQL) check it first.
func Active() bool {
	p := current.Load()
	return p != nil && p.limits().Enabled
}

// Emit redacts e and queues it for publishing. It never blocks and never
// fails the caller: with no publisher, the tap off, or the buffer full, the
// event is discarded (the last is counted).
func Emit(ctx context.Context, e Event) {
	p := current.Load()
	if p == nil {
		return
	}
	lim := p.limits()
	if !lim.Enabled {
		return
	}
	if e.T.IsZero() {
		e.T = time.Now().UTC()
	}
	if e.N == 0 {
		e.N = 1
	}
	if isUser(ctx) {
		e.Label = glyphs()
		if e.Payload != "" {
			e.Payload = glyphs()
		}
		e.Redacted = true
	} else {
		e.Label = clean(e.Label, lim.MaxLabelChars)
		e.Payload = clean(e.Payload, lim.MaxLabelChars)
	}
	p.enqueue(e, lim.BufferSize)
}

// glyphAlphabet holds the symbols redacted text is drawn from.
var glyphAlphabet = []rune("◈◇◆○●□■△▲▽▼◊※⌘⌖⊕⊗⊙⟁⟐⧫⬡⬢")

// glyphMin and glyphMax bound a redacted string's length. It is random, not
// the original's length, so a redacted label says nothing about its text.
const glyphMin, glyphMax = 3, 12

func glyphs() string {
	n := glyphMin + rand.IntN(glyphMax-glyphMin+1)
	var b strings.Builder
	for range n {
		b.WriteRune(glyphAlphabet[rand.IntN(len(glyphAlphabet))])
	}
	return b.String()
}

// Symbols returns random symbols for the part of a public label that must
// never be shown (a rate-limit key), whoever the event belongs to.
func Symbols() string { return glyphs() }

// clean makes public text safe to put on one line of the stream: control
// characters become spaces, runs of whitespace collapse, and anything past
// max runes is cut with an ellipsis. Viewers still escape it — an issue
// title is attacker-controlled.
func clean(s string, max int) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	space := false
	n := 0
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if n >= max {
			b.WriteRune('…')
			return b.String()
		}
		if space {
			b.WriteByte(' ')
			space = false
			n++
			if n >= max {
				b.WriteRune('…')
				return b.String()
			}
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
