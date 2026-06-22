module github.com/anumey1/Suns

go 1.22

toolchain go1.26.4

// Dependencies are intentionally not yet required: the scaffold is stdlib-only
// so it compiles before the toolchain is set up. They will be added by
// `go get` / `go mod tidy` as each package starts using them (§2):
//
//   github.com/spf13/cobra              command & flag routing
//   github.com/spf13/viper              config load (once, into SessionState)
//   github.com/charmbracelet/bubbletea  TUI runtime (Elm architecture)
//   github.com/charmbracelet/bubbles    prebuilt TUI components
//   github.com/charmbracelet/lipgloss   styling & layout
//   github.com/charmbracelet/harmonica  spring-physics easing
//   github.com/NimbleMarkets/ntcharts   braille charts (Apple_Terminal fallback)
//   github.com/lrstanley/bubblezone     mouse zones
//   github.com/shirou/gopsutil/v4       in-process system stats
//   golang.org/x/sys/unix               direct syscalls (openat/unlinkat/statfs/sysctl)
//   howett.net/plist                    binary+XML plist parsing
