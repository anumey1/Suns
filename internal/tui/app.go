// Package tui is the presentation layer: a Bubble Tea Model-Update-View root
// that routes between views and renders from immutable snapshots (§3.2, §10).
//
// A tea.Tick (~10 Hz) does a lock-free atomic.Pointer.Load of the latest
// double-buffered snapshot and renders it; the render loop never blocks, never
// holds a lock across heavy work, and never allocates a fresh giant data
// structure per frame. The model emits tea.Cmd to start/cancel jobs and to
// confirm a sealed plan.Plan. No engine is ever imported the other way around —
// dependencies flow downward only (§3.1).
package tui

// App is the root Bubble Tea model and the router between views (menu, clean,
// nuke, dashboard, audit, net, scan, restore, …).
type App struct{}

// TODO(phase1): implement tea.Model (Init/Update/View), window-size reflow,
// the focus model, mode recoloring, and the frozen-plan confirmation modal.
