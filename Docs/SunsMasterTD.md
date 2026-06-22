# **Suns — Super User Nutcase Sessions**

## **Master Technical Document (v6 — Single Source of Truth)**

**Tagline:** *Burn It All.*

A high-performance, beautifully crafted terminal-UI (TUI) system utility for macOS, written in Go. Suns unifies four capabilities behind one cohesive, safety-obsessed interface:

1. A **safe, auditable cleanup & operation engine** (caches, logs, dev artifacts, duplicates, empty directories, broken symlinks).  
2. A **precision uninstaller** that removes apps and their traces without orphaning files or breaking shared dependencies.  
3. A **deep system & security auditor** (SIP/Gatekeeper/FileVault posture, open ports, sockets, auth-log analysis, LAN discovery).  
4. A **live `get-coffee` telemetry dashboard** with real-time, widget-based monitoring of the whole machine.

**Document status:** This is the **final, authoritative master document**. It supersedes blueprints v1–v5 and consolidates every decision, correction, mechanism, and feature discussed across all five prior versions and their review rounds. It is intended to be sufficient, on its own, to build Suns correctly and completely. Where earlier versions compacted detail (notably v2–v5), this version re-expands everything to full elaboration.

**Target platform:** macOS 13 Ventura → macOS 15 Sequoia. **Apple Silicon is the primary target; Intel is supported.** **Language / runtime:** Go 1.22 or newer. **Distribution:** Universal binary, code-signed (Developer ID) and notarized, delivered via signed `.pkg` and a Homebrew tap.

---

## **0\. How This Document Came To Be (Evolution & Rationale)**

Suns has been through five complete blueprint iterations and several adversarial review rounds. This history matters because many decisions in this document are the *resolution* of a specific failure mode discovered in review, and understanding the "why" prevents a future maintainer from reintroducing the bug.

* **v1 (original Gemini draft):** Established the core idea — a Go TUI macOS cleaner with a two-axis safety model (Deathstar/Jarjar). Contained several incorrect or stale technical choices: a filesystem-event library mistaken for a trash library, an archived directory-walking dependency, an assumption that all plists are XML, "kill \-9 a zombie" (impossible), `git gc --aggressive --prune=now` across all repos (data loss), cron scheduling (deprecated on macOS), Intel-only SMC thermal reads, and the deprecated `/var/log/system.log`. It also never mentioned macOS permissions (TCC / Full Disk Access) at all, and treated the TUI as an afterthought.

* **v2:** Fixed the stale dependencies, introduced the macOS permission reality (TCC/FDA), a real privilege model, a curated safe-cache allowlist, and a genuine TUI design system. Added the `get-coffee` live dashboard concept.

* **v3:** Recognized that the safety model was **path-centric while the product was not** — it could only describe file deletion, not process kills, DNS flushes, Docker prunes, or service unloads. Introduced the **generalized Operation model** with typed action kinds and reversibility classes. Added execution-time revalidation (TOCTOU defense), hardened command execution, stream-based telemetry, APFS-aware dedup, neutral machine-facing config/log keys (while keeping the fun names in the UI), and macOS-native log locations.

* **v4:** Hardened the *mechanisms* behind v3's concepts: bounded Cgo for the trash call (to prevent OS-thread exhaustion), a simplified-and-correct privilege model (the XPC helper was demoted because Go has no native XPC and helpers fit app bundles, not Homebrew CLIs), frozen execution plans, fd-anchored recursive deletion, identity-aware validation, supervised telemetry, and the demotion of the "audit" log to an honest "operation history."

* **v5:** Closed the remaining runtime-behavior gaps that v4's mechanisms still had: the trash circuit breaker that **bricked the subsystem** (fixed with graceful pure-Go fallback), the volume probe that could itself block in-kernel (fixed with a killable helper subprocess), GC thrash from snapshot churn (fixed with double-buffering), a payload harvester that could delete **shared libraries** (fixed with `pkgutil --file-info` guarding), a process-validation step that couldn't run under its own privilege rules (fixed by delegating to the privilege chokepoint), full-content-hash restore that blocked for minutes (fixed with tiered identity), a telemetry tokenizer that could stall mid-document (fixed with a read-deadline), firmlink traversal that falsely rejected `/var` and `/Users` (fixed with a firmlink-aware identity layer), aspirational plan immutability (fixed with value-sealed plans), and an under-specified persistent root runner (dropped entirely).

* **v6 (this document):** Consolidates all of the above into one fully-elaborated source of truth, with every feature, mechanism, and cross-cutting concern detailed at implementation depth.

A consolidated table of every review item and its disposition is in **Appendix 16.1**.

---

## **Table of Contents**

1. [Vision, Principles & Product Scope](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#1-vision-principles--product-scope)  
2. [Technology Stack (with full justification)](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#2-technology-stack)  
3. [System Architecture](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#3-system-architecture)  
4. [The Operation Model & Safety Engine](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#4-the-operation-model--safety-engine)  
5. [macOS Platform Reality: Permissions, Privilege, Signing, Hardware](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#5-macos-platform-reality)  
6. [The Privilege Model](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#6-the-privilege-model)  
7. [Telemetry Architecture](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#7-telemetry-architecture)  
8. [Project Layout](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#8-project-layout)  
9. [Command Taxonomy](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#9-command-taxonomy)  
10. [The TUI: Design System, UX & Aesthetics](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#10-the-tui-design-system-ux--aesthetics)  
11. [The `get-coffee` Live Telemetry Dashboard](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#11-the-get-coffee-live-telemetry-dashboard)  
12. [Feature Specifications (all engines, fully detailed)](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#12-feature-specifications)  
13. [Cross-Cutting Concerns](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#13-cross-cutting-concerns)  
14. [Implementation Roadmap & Product Scope](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#14-implementation-roadmap--product-scope)  
15. [Testing & Safety Validation](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#15-testing--safety-validation)  
16. [Appendices](https://claude.ai/chat/ddfd9005-1f6a-4318-9027-2c00ce1b2a62#16-appendices)

---

## **1\. Vision, Principles & Product Scope**

### **1.1 What Suns is, and who it is for**

Suns is a **power-user system utility**. It assumes a competent operator — a developer or advanced macOS user — and gives them genuinely sharp tools: the ability to permanently obliterate files, kill system processes, prune containers, and reset OS subsystems. But it is built on the conviction that *sharp tools must never cut the user by accident*. Every capability is wrapped in a layered safety apparatus, and the default behavior of the program — with no flags and no configuration — cannot permanently destroy data.

The intended user runs a terminal-first workflow, values speed and density of information, dislikes GUI bloat, and wants a single tool that replaces a drawer full of shell scripts, `brew cleanup`, manual `rm -rf`, Activity Monitor, and a half-dozen menu-bar monitors — without sacrificing safety or trust.

### **1.2 The non-negotiable design principles**

These principles are the constitution of the project. Every later section is an application of one or more of them, and no feature may violate them.

1. **Safe by default, dangerous on demand.** A first run, with zero flags and zero configuration, can never permanently destroy data. The safe defaults are: preview-and-confirm before acting (`confirm_mode` off → the gate is shown), and move-to-Trash rather than permanent deletion (`deletion_mode` \= trash). Destruction is always an explicit, deliberate escalation that the operator opts into.

2. **Everything destructive is a typed Operation, not merely a file deletion.** Killing a process, flushing DNS, pruning Docker, unloading a launch agent, garbage-collecting a git repo — each is a first-class, typed, audited action with its own preview shape, its own reversibility classification, its own identity-aware validation, and its own history record. The safety machinery is polymorphic over operation kinds, not hardcoded to files.

3. **Read-only and destructive operations are categorically different.** They live in different command groups, render in different colors, and only destructive ones pass through the confirmation gate. A monitor or auditor never prompts and never deletes; a destructive command always can.

4. **Approve exactly what executes — no drift.** The operator confirms a *frozen, value-sealed plan*; the executor consumes that exact same sealed plan, revalidated at execution time. What is shown in the preview and what is actually done can never diverge structurally.

5. **The OS is the boss.** Suns respects System Integrity Protection (SIP), Transparency-Consent-and-Control (TCC) / Full Disk Access, code signing, firmlink volume separation, and sandbox boundaries, rather than fighting them. When it lacks a permission, it says so clearly and degrades gracefully — it never fails silently or presents a partial result as complete.

6. **Nothing happens invisibly.** Every destructive operation is previewed by default, revalidated at execution time, recorded in an operation-history log, and classified by how reversible it is. The operator always knows what was done and whether it can be undone.

7. **The terminal deserves real design.** A TUI can be as polished, legible, responsive, and pleasant as any GUI. Typography, spacing, color, motion, and information hierarchy are first-class concerns, not afterthoughts. The target quality bar is `lazygit`, `k9s`, and `btop`.

8. **Concurrency without coupling, without lying, without thrashing.** Heavy I/O and system calls run on worker goroutines and surface to the UI through lock-free, double-buffered immutable snapshots; the render loop is never blocked, never holds a lock across heavy work, and never allocates a fresh giant data structure on every frame. Performance targets are stated as *goals*, not guarantees, because a TUI that also drives heavy system binaries can stall.

9. **Native calls degrade, never brick.** Any call into C (the Trash API, IOKit sensors) is resource-bounded *and* has a graceful fallback, so that a hung volume or a wedged sensor can neither crash the Go runtime (by exhausting OS threads) nor disable the feature on healthy resources.

10. **Honesty about guarantees.** The document and the program describe what the mechanisms actually deliver. The operation history is an *operational record*, not a tamper-evident forensic trail, and is described as such. Per-process network attribution is *best-effort*, and is labeled experimental. "Never blocked" and frame-rate figures are *intentions*, not promises.

### **1.3 Product scope — a deliberately minimal, trustworthy v1**

A destructive systems utility has an enormous correctness surface, and across five review rounds the breadth of the v1 feature set was repeatedly (and correctly) flagged as a risk: cleaner \+ uninstaller \+ dedup \+ telemetry \+ network \+ audit \+ scheduling \+ restore \+ Docker \+ git in one product is a great deal of code that can go wrong in a tool whose mistakes delete data.

Therefore the **shipping order deliberately front-loads the proven-safe core** and defers the high-correctness-surface features. The architecture in this document supports the entire product, but the build sequence (detailed in §14) is:

* **v1 — Trust the core.** The complete safety spine (operation model, frozen plan, fd-anchored deletion, bounded-and-degrading trash, identity validation, operation history, deny floor, privilege chokepoint, config/state), the `get-coffee` dashboard (entirely read-only, requires no Full Disk Access, and demonstrates the full UX with zero destructive risk), and `clean` restricted to the **curated safe-cache allowlist** (dev caches and empty directories). Plus `restore` for undo. This is a complete, beautiful, low-risk product on its own.  
* **v2 — Earned breadth.** The uninstaller (with the shared-dependency guard), duplicate finder (APFS-aware), and restore hardening.  
* **v3 — Inspection suites.** Network inspection and security audit suites.  
* **v4 — Power features.** Scheduling, Docker prune, git maintenance, the localization stripper (quarantined), and the full release/distribution pipeline.

---

## **2\. Technology Stack**

Every dependency below is chosen deliberately, and several choices are corrections of mistakes from earlier blueprint versions. The justification column explains *why* — including, where relevant, what was wrong before.

### **2.1 Core language & CLI**

* **Go 1.22+** — Chosen for: native concurrency primitives (goroutines, channels, `context`), a single statically-linked binary, excellent cross-compilation, generics, the `slices`/`maps` standard packages, and the corrected `for`\-loop variable semantics introduced in 1.22 (which eliminate a whole class of closure-capture bugs in worker pools). Cgo is available where macOS forces it, but is used sparingly and always with a non-Cgo fallback.

* **`github.com/spf13/cobra`** — Command and flag routing. Gives the `suns <command> <subcommand> --flags` structure, automatic help generation, and shell completion.

* **`github.com/spf13/viper`** — Configuration file management, **read exactly once at startup**. *Critical correction:* Viper is **not safe for concurrent read/write**. A background worker calling `viper.GetBool("...")` at the same millisecond the user toggles a setting would race and can panic. Therefore Viper is read once into a mutex-guarded Go struct (`SessionState`), and engines never touch Viper at runtime (see §4.9).

### **2.2 The TUI stack (the Charm ecosystem)**

* **`github.com/charmbracelet/bubbletea`** — The TUI runtime, implementing The Elm Architecture (Model-Update-View). All UI state lives in a model; messages drive pure update functions; the view renders from state. This functional structure is what makes the UI testable and keeps it decoupled from the engines.

* **`github.com/charmbracelet/bubbles`** — Prebuilt components: `table` (sortable, scrollable), `viewport` (scrollable region), `progress`, `spinner`, `list`, `textinput`, `paginator`, `help`, and the `key` binding map. These save substantial work and provide consistent behavior.

* **`github.com/charmbracelet/lipgloss`** — Styling and layout: borders, padding, color (including `AdaptiveColor` for light/dark terminals), and flexbox-like vertical/horizontal joins for composing panels.

* **`github.com/NimbleMarkets/ntcharts`** (with an ASCII-block fallback) — Braille-based line charts and sparklines for the dashboard. *Critical correction:* Apple's default Terminal.app has documented bugs rendering Unicode Braille at certain line-heights, causing charts to tear or stagger vertically. Therefore the chart component detects `TERM_PROGRAM == Apple_Terminal` and **falls back to block elements (`▁▂▃▄▅▆▇█`)** to guarantee visual fidelity. (`guptarohit/asciigraph` is an acceptable alternative charting library.)

* **`github.com/lrstanley/bubblezone`** — Mouse "zones" that make widgets and menu items clickable, layered on top of keyboard control (which remains primary).

* **`github.com/charmbracelet/harmonica`** — Spring-physics animation/easing, used so gauges and transitions *ease* to their target values rather than jumping, within a strict motion budget and behind a `--reduce-motion` flag.

### **2.3 System telemetry & low-level access**

* **`github.com/shirou/gopsutil/v4`** — In-process system statistics: CPU (aggregate and per-core), memory, disk usage and I/O counters, host info, and process enumeration. Cheap and reliable for these. *Important limitation:* gopsutil's **per-process network counters are unreliable on Darwin** (largely a Linux capability), so per-PID network attribution is *not* taken from gopsutil (see below and §7.5).

* **`golang.org/x/sys/unix`** — Direct syscall access for the things the standard library doesn't expose safely: `openat`/`unlinkat`/`fstatat` (file-descriptor-relative traversal for the safe recursive deleter), the `O_NOFOLLOW` flag, `statfs`/`MNT_*` mount flags (firmlink detection), `sysctl` reads (e.g. `net.inet.tcp.pcblist` for network stats), and `removefile`.

* **stdlib `io/fs` / `filepath.WalkDir`** — Directory discovery (read-only walking). *Critical correction:* the original blueprint used `github.com/karrick/godirwalk`, which the author has since **archived** and explicitly recommends replacing with `filepath.WalkDir` (introduced in Go 1.16). `WalkDir` uses `os.DirEntry` and avoids the per-entry `lstat` that made the old `filepath.Walk` slow, so the performance argument for the third-party library no longer holds. *Equally critical:* `WalkDir` is used **only for discovery**, never for deletion — deletion uses the fd-anchored deleter (§4.6).

### **2.4 Parsing, hashing, and native macOS integration**

* **`howett.net/plist`** (with a `plutil` shell fallback), plus a **custom streaming tokenizer** for `powermetrics` — Property-list parsing. *Critical correction:* the original blueprint assumed all `Info.plist` files are XML. Many modern plists are **binary**, which `encoding/xml` cannot parse; `howett.net/plist` handles both. *Second correction (telemetry):* `powermetrics -f plist` emits a **concatenated stream** of discrete plist documents, not one well-formed document; feeding that stream directly to a single-document decoder fails at the first boundary. A custom tokenizer buffers one complete `<?xml … </plist>` block at a time (with a read-deadline; see §7.3).

* **stdlib `crypto/sha256` \+ `hash/crc32`**, used in a **tiered** scheme — File content hashing for duplicate detection and restore-identity verification. *Critical correction:* hashing a 40 GB cache in full to verify identity before restoring it would block for many minutes; therefore identity is **tiered** — large files (above a threshold, e.g. 50 MB) are identified by size \+ mtime \+ inode (no full read), and only small, high-risk files get a full SHA-256 (see §4.7 and §4.8). For dedup, a cheap first-4 KB hash is a middle pass before the full hash.

* **Cgo → `NSFileManager trashItemAtURL:`**, **bounded and degrading** — Native macOS Trash. *Critical correction (compounded across versions):* the original cited `rjeczalik/notify`, which is a filesystem-*event* library (FSEvents wrapper), not a trash library. The correct mechanism is the Cocoa Foundation call. Moreover, because any Cgo call pins an OS thread, a hung Trash call (dead SMB mount, stalled FSEvents) can exhaust the Go runtime's thread limit and crash the program; therefore the call is bounded by a semaphore and a circuit breaker, fronted by a killable volume-reachability probe, and **degrades to a pure-Go `~/.Trash` move** when bounded out (see §4.4 — this is one of the most important mechanisms in the entire document).

### **2.5 Privilege & logging**

* **Hardened `syscmd` \+ a centralized per-workflow `sudo` chokepoint** — Privileged execution. *Critical correction:* v3 proposed a signed `SMAppService` \+ XPC helper as the primary escalation path; this was demoted because Go has no native XPC bindings (the Cgo surface bridging Go's garbage collector against XPC's manual reference counting is volatile), and helper approval fits an app-bundle/installer world rather than a bare CLI shipped via Homebrew. v4's alternative of a "persistent elevated runner" was then dropped because a long-lived root process with a command channel is itself an ambient-root control surface. The final model is a single chokepoint that acquires a per-workflow `sudo` ticket (see §6).

* **stdlib `log/slog`** (application log) \+ a **custom JSONL operation-history log** (`0600`) — Two distinct logs. The slog application log (`~/Library/Logs/Suns/suns.log`) is for diagnostics. The operation-history log (`~/Library/Application Support/Suns/history.jsonl`) is the record of destructive actions and the source of truth for restore. *Correction:* the history log is **not** called an "audit" log, because it has no tamper-evidence; calling it that would overstate its guarantees (see §4.10, §13.3).

### **2.6 Dependency philosophy**

Prefer the standard library, the `golang.org/x/sys` package, and the Charm ecosystem. Pull in Cgo only where macOS genuinely forces it (the Trash API and IOKit thermal sensors), and always provide a non-Cgo fallback so the build degrades gracefully. Every scraped external command (`lsof`, `nettop`, `powermetrics`, etc.) is wrapped behind the hardened `syscmd` layer with a versioned parsing contract (§13.1), never called ad hoc.

### **2.7 Rejected technologies (and why)**

These were proposed in review rounds and **deliberately rejected**; they are recorded here so they are not reconsidered without cause.

* **`rjeczalik/notify` for Trash** — it is a filesystem-event library, not a trash library. Wrong tool.  
* **`godirwalk`** — archived by its author; superseded by stdlib `filepath.WalkDir`.  
* **A remote-updatable IOKit sensor manifest** — adds a network fetch and update-trust surface to a security-sensitive tool; the sensor manifest is **embedded** and ships with releases instead.  
* **`sandbox-exec` profiling for FDA detection** — vaguer and less reliable than simply attempting the operation and handling per-path `EPERM`.  
* **A `.DS_Store`\-written "Put Back" metadata hack for headless trashing** — technically wrong; Put-Back metadata lives in extended attributes / the Trash subsystem, not in `.DS_Store`. The correct behavior is to refuse on failure rather than fake metadata.  
* **A Go arena allocator for snapshot building** — Go's arena experiment is not stable; the double-buffering technique (§3.2) achieves the same zero-allocation goal with stable APIs.  
* **A persistent elevated (root) runner process** — an unspecified ambient-root control surface; replaced by per-workflow `sudo` tickets (§6).  
* **XPC/SMAppService as the *primary* privilege path** — impractical from Go and ill-suited to Homebrew distribution; retained only as an optional future path behind the same chokepoint if an app-bundle distribution is ever pursued.

---

## **3\. System Architecture**

Suns is a **modular monolith**: one binary, but internally organized into strictly-layered packages with a single dependency direction. This gives the simplicity of one deployable artifact with the testability and clarity of well-separated concerns.

### **3.1 The dependency rule**

Dependencies flow **downward only**: `cmd → internal/cli → internal/<engines> → pkg/<platform>`. The platform layer (`pkg/*`) imports nothing from above it. No engine ever imports the TUI. This is enforced and means:

* Every engine is independently unit-testable without a terminal.  
* Every engine is usable headlessly from the CLI (for scripting and CI).  
* The safety primitives in `pkg/safety`, `pkg/operation`, `pkg/plan`, `pkg/trash`, and `pkg/history` have no knowledge of how they are presented, so the same gate logic backs both the CLI text preview and the TUI confirmation modal.

### **3.2 The three layers**

┌──────────────────────────────────────────────────────────────────────┐  
│  PRESENTATION LAYER  (internal/tui)                                    │  
│  Bubble Tea Model-Update-View · lipgloss rendering · widget components │  
│  A tea.Tick (\~10 Hz) does a lock-free atomic.Pointer.Load of the       │  
│  latest immutable snapshot and renders it. Emits tea.Cmd to start or   │  
│  cancel jobs and to confirm a sealed Plan.                             │  
└───────────────▲──────────────────────────────────────┬────────────────┘  
        load \*Snapshot (lock-free, double-buffered)  start/cancel ctx,  
                │                                     confirm sealed Plan  
┌───────────────┴──────────────────────────────────────▼────────────────┐  
│  DOMAIN / ENGINE LAYER  (internal/scanner, uninstaller, dedup,         │  
│  optimizer, telemetry, audit, net, scheduler)                          │  
│  Context-cancellable jobs. Workers fill the INACTIVE buffer of a       │  
│  double buffer; a builder sorts it in place and atomically swaps the   │  
│  pointer. Engines produce a frozen Plan and later consume that Plan.   │  
│  Pure logic; no UI imports.                                            │  
└───────────────▲──────────────────────────────────────┬────────────────┘  
                │                                        │  
┌───────────────┴──────────────────────────────────────▼────────────────┐  
│  PLATFORM / PKG LAYER  (pkg/\*)                                         │  
│  operation (the model) · plan (Seal value-copy) ·                      │  
│  safety (gate, deny-floor, revalidate, fsdelete, firmlink) ·           │  
│  trash (bounded Cgo \+ pure-Go fallback) · history (0600 JSONL \+        │  
│  export-redaction) · privilege (sudo chokepoint) · syscmd (hardened    │  
│  exec \+ parsing contracts) · plist (howett \+ plutil \+ tokenizer) ·     │  
│  theme (palettes, mode accents)                                        │  
└────────────────────────────────────────────────────────────────────────┘

### **3.3 The UI update model — double-buffered immutable snapshots**

This is the result of two successive corrections. The original design streamed one message per discovered file into Bubble Tea's sequential update loop; under a large scan this produces hundreds of thousands of messages and saturates the loop, spiking memory and lagging the UI. The first fix (v4) was a pull model: workers update shared state behind a read-write mutex, and the UI ticks and reads a snapshot. But review showed that holding a read-lock while sorting and paging 250,000 results blocks all writer workers — inverting the backpressure and stalling the scanner — and that rebuilding a fresh 250,000-element slice every tick generates heavy heap allocation and stop-the-world GC pauses that stutter the very UI the model was meant to keep smooth.

The final design (v5/v6) is **double-buffering with atomic pointer swaps**:

* The engine preallocates **exactly two** view arrays (a double buffer). The builder goroutine writes and sorts the **inactive** buffer **in place**, reusing its backing array, then publishes it with a single `atomic.Pointer.Store`. The UI reads the **active** buffer via a lock-free `atomic.Pointer.Load`.  
* **Per-tick allocation is approximately zero at steady state** — the backing arrays are reused and only grow when the result count exceeds current capacity. This eliminates the GC thrash.  
* A **snapshot size cap** (for example, top-N by the active sort key, plus an overflow count) bounds the rendering and sorting work regardless of how many total matches exist. The complete result set lives separately in the engine's append-only store, which is what the frozen plan is built from — not the per-tick view.  
* For very large scans the builder rebuilds on a **coarser cadence** (on a milestone, or at most twice per second) while the UI still ticks at roughly 10 Hz, reading whatever the latest buffer holds.  
* No lock is ever held across a sort or a render. Readers never see a partial state (the pointer swap is atomic); writers never wait on readers.

Note on cadence: 10 Hz is more than adequate for a scan progress counter; a 60-frames-per-second target is explicitly **not** pursued for scan progress. Dashboard telemetry snapshots update at 1–2 Hz, which is the rate at which the underlying data meaningfully changes.

### **3.4 The engine job contract**

Every long-running engine operation implements a uniform shape so the TUI treats them identically:

// A Job streams progress and results until its context is cancelled or  
// the work completes. It MUST select on ctx.Done() at every directory  
// boundary and between work batches, and return promptly on cancellation.  
type Job interface {  
    Run(ctx context.Context) \<-chan Event  
}

type Event interface{ isEvent() }

type ProgressEvent  struct { Done, Total int; Current string }   // coalesced  
type MatchEvent     struct { Path string; Size int64; Category string }  
type TelemetryEvent struct { Snapshot \*SystemSnapshot }          // get-coffee  
type DoneEvent      struct { Summary Summary }  
type ErrorEvent     struct { Err error; Path string; Fatal bool } // e.g. a TCC denial

In Bubble Tea, the channel is drained by a `tea.Cmd` that reads one event, converts it to a `tea.Msg`, and re-issues itself. Importantly, for large scans the engine does **not** emit a `MatchEvent` per file to the UI; matches accumulate in the engine store and the UI reads the double-buffered snapshot. The channel carries only coarse progress, completion, and error events.

### **3.5 Concurrency, cancellation, and backpressure**

* **Bounded worker pool.** Size is `min(runtime.NumCPU(), 8)`, configurable. Workers consume directory roots from a buffered queue and walk them with `filepath.WalkDir`.  
* **One shared context.** All workers share a single `context.Context`. Pressing `q`, `esc`, or `ctrl+c` (TUI) or sending `SIGINT` (CLI) cancels it; every worker checks `ctx.Done()` at each directory boundary and between file batches and exits promptly.  
* **No blocked senders.** Because there is no per-file UI channel, workers never block waiting for the UI to consume. The only channel is a small buffered completion/error channel drained by the UI tick.  
* **No orphaned goroutines.** A `sync.WaitGroup` tracks workers; the engine's `Run` returns only after every worker has observed cancellation and exited. This is verified in tests with `goleak`.  
* **Native-call bounding** (the one place a worker calls into C, the Trash API) is handled by the dedicated mechanism in §4.4, independent of the worker pool.

---

## **4\. The Operation Model & Safety Engine**

This section is the heart of Suns. It is long because the safety machinery is the reason the product can be trusted, and because several of its mechanisms are the resolution of specific, subtle failure modes.

### **4.1 Why a generalized operation model**

The original safety model was built entirely around file paths: a target was a path, the only choice was "trash versus permanently delete," the log recorded a path and its fate, and "undo" meant putting the path back. This is correct for file deletion and *wrong for everything else the product does*. Consider the actual destructive surface:

| Action | Has a file-path target? | Maps to "trash vs obliterate"? | Reversible from a path log? |
| ----- | ----- | ----- | ----- |
| Delete a cache file | Yes | Yes | Yes (restore) |
| Kill a process | No (a PID) | No | No (it is gone) |
| Flush DNS cache | No | No | No (nothing to undo) |
| `git gc` | Partially | No | Only via reflog grace window |
| Docker prune | No (images/volumes) | No | No |
| `launchctl bootout` | Partially | No | Possibly (re-load) |

A path-centric model either cannot represent these actions or, worse, misrepresents them — implying an "undo" or a "trash" semantic that does not exist. So Suns models **Operations**, and the safety apparatus is polymorphic over operation kinds.

### **4.2 The Operation interface**

// Operation is any auditable, potentially-destructive action.  
type Operation interface {  
    Kind() OpKind                          // FileDelete, ProcessKill, CacheReset,  
                                           // ServiceUnload, RepoMaintenance,  
                                           // ContainerPrune, ReceiptForget, DNSFlush, ...  
    Describe() Preview                     // what the gate renders for this op  
    Reversibility() Reversibility          // Reversible | Recoverable | Irreversible  
    ValidateAtPlan(ctx context.Context) error          // discovery-time check  
    ValidateAtExec(ctx context.Context) (Identity, error) // execution-time identity recheck  
    Execute(ctx context.Context, mode Mode, id Identity) (Receipt, error)  
    HistoryRecord(Receipt) HistoryEntry    // typed record for this op kind  
}

type OpKind string  
type Reversibility int  
const (  
    Reversible   Reversibility \= iota // e.g. a trashed file → restore  
    Recoverable                       // e.g. git gc within reflog window; a cache that rebuilds  
    Irreversible                      // e.g. a killed process; an obliterated file; a DNS flush  
)

Each operation:

* **`Describe()`** returns a per-kind preview — a file table for `FileDelete`, a process table for `ProcessKill`, an image/volume list for `ContainerPrune`, a single explanatory line for `DNSFlush`.  
* **`Reversibility()`** returns one of three honest classes, surfaced in the gate with an unmistakable badge: 🟢 Reversible, 🟡 Recoverable, 🔴 Irreversible.  
* **`ValidateAtPlan`** checks the operation is sensible at discovery time.  
* **`ValidateAtExec`** re-checks, at execution time, that the target is still exactly what was planned, returning a typed `Identity` that `Execute` refuses to act without (this is the TOCTOU and PID-reuse defense; see §4.7).  
* **`HistoryRecord`** produces a typed log entry appropriate to the kind (a file delete records paths and fate; a process kill records PID, name, and signal; see §4.10).

The gate (§4.4) operates on a slice of `Operation`, groups them by kind, and renders the appropriate preview and reversibility badge per group, so the operator sees *exactly* what kind of harm each group can do before confirming.

### **4.3 The two safety axes**

Two orthogonal toggles control behavior. They have memorable UI names and neutral machine-facing keys (the machine keys are what appear in config files and logs, for scriptability and clarity; see §13.3).

| Axis | UI label | Machine key | Question it answers | Default | Scope |
| ----- | ----- | ----- | ----- | ----- | ----- |
| Confirmation gate | **Deathstar** | `confirm_mode` | Do I confirm before acting? | off → preview \+ `[y/N]` gate | All operations |
| Deletion method | **Jarjar** | `deletion_mode` | Where do deleted *files* go? | `trash` | **`FileDelete` only** |

`confirm_mode` off means the gate is shown and the operator must approve; `confirm_mode` on means immediate execution with a post-hoc history record. `deletion_mode` \= `trash` moves files to the macOS Trash (recoverable); `deletion_mode` \= `obliterate` permanently deletes them.

**Crucial scoping rule:** the deletion axis applies **only** to `FileDelete` operations. You cannot "trash" a killed process or a flushed DNS cache. Non-file operations ignore the deletion axis entirely and always display their true reversibility — a `ProcessKill` is shown as 🔴 Irreversible regardless of the deletion-mode setting. This ensures the model never implies an undo it cannot deliver.

The four states for file deletion specifically:

| `confirm_mode` | `deletion_mode` | Behavior | Risk |
| ----- | ----- | ----- | ----- |
| off | trash | Preview → confirm → move to Trash | 🟢 Safest (the default) |
| off | obliterate | Preview → confirm → permanent delete | 🟡 Confirmed destruction |
| on | trash | Immediate move to Trash, no prompt | 🟡 Fast but recoverable |
| on | obliterate | Immediate permanent delete, no prompt | 🔴 "Burn It All" |

In the TUI, the active `confirm_mode` recolors the entire interface chrome (cool plasma-blue when safe, ember-red when armed) and an independent chip shows the deletion mode (`🗑 TRASH` green vs `🔥 OBLITERATE` red), so the operator's safety posture is unmistakable at a glance.

### **4.4 Bounded and degrading native Trash**

This is one of the most important mechanisms in the document, and it is the resolution of a cascade of failure modes discovered across versions.

**The problem chain.** The correct way to move a file to the macOS Trash (so it has working "Put Back" and proper Trash semantics) is the Cocoa call `NSFileManager trashItemAtURL:`, reached via Cgo. But every Cgo call **pins an OS thread** for its duration. If that call hangs — a disconnected SMB/AFP network share, a stalled FSEvents queue, a pathological mount — the OS thread is lost to the Go runtime and never returned. A naive design that trashes 500 files on a hung volume can spawn up to 500 permanently-blocked threads, and the Go runtime has a hard ceiling (around 10,000 threads) beyond which the program **fatally crashes**. v4 attempted to bound this with a semaphore and a circuit breaker, but review found that this merely traded a crash for a **permanently bricked subsystem**: once N calls are stuck and the breaker trips, trashing is dead for the rest of the session, so even a file on a healthy local volume can no longer be trashed. And the proposed volume-reachability probe (`statfs`) could itself **block in the kernel** on a dead mount — a Go-side timeout cannot cancel a syscall stuck in the kernel.

**The final, layered mechanism (in `pkg/trash`):**

1. **Volume-reachability probe in a killable helper.** Before a trash batch, targets are grouped by volume. Reachability is checked with `statfs`/access — but run inside a **short-lived helper subprocess** that the parent can actually `kill` if it hangs (because a Go goroutine blocked in an in-kernel `statfs` cannot be cancelled, but a child process can be killed). If spawning a helper is unavailable in some context, the design **explicitly accepts bounded thread loss** for the probe and relies on the breaker and fallback below — it does not pretend a timeout solves an uncancellable syscall. An unreachable volume causes its entire batch to be **skipped and reported**, never attempted natively.

2. **Concurrency semaphore.** At most a small number `N` (for example 4\) of trash Cgo calls are in flight simultaneously, each isolated in its own goroutine with a per-call timeout. This caps the number of threads that can be simultaneously pinned by trashing.

3. **Circuit breaker that degrades, never refuses.** After `K` consecutive trash-call timeouts, or when the semaphore is saturated by stuck calls, the `FileDelete` operation **does not refuse** — it **gracefully degrades to a pure-Go fallback** that moves the item into the correct Trash directory without the Cocoa API:

   * If the item is on the same volume as `~/.Trash`, use `os.Rename` into `~/.Trash`.  
   * If it is on a different volume, use that volume's `/.Trashes/<uid>` directory if writable; otherwise **copy-then-unlink** (because `os.Rename` fails with `EXDEV` across volume boundaries). It never silently performs a permanent delete as a "fallback."  
   * The fallback has **no OS "Put Back" metadata** — and that is acceptable, because **restore uses Suns's own operation-history log, not the OS Put-Back facility** (see §4.8). Recoverability is therefore unchanged whether a file was trashed via Cgo or via the fallback.  
   * **The consequence:** a healthy local volume **always** succeeds, even while a dead SMB mount has tripped the breaker. The breaker stops native calls only to the *bad* volume; it never disables trashing globally. The subsystem degrades but never bricks.  
4. **IOKit containment (the same discipline for the other Cgo path).** Principle 9 says *any* native call is bounded, and the thermal-sensor path also uses Cgo (IOKit). Each IOKit sensor read runs under a per-call timeout in a bounded goroutine, supervised by the telemetry supervisor (§7.4). A hung sensor read marks the thermal source `stale`/`unavailable` and is not retried in a tight loop, so IOKit Cgo calls cannot accumulate without bound either.

### **4.5 The frozen, value-sealed plan**

**The problem.** The UI's snapshot is a *paged, capped, cosmetic view* — it might show the top 500 of 250,000 matches. If the gate confirmed "whatever the UI is currently displaying" and the executor acted on "whatever the engine's live aggregate currently holds," the operator could approve a summarized subset while the engine acts on a larger set, and planning could structurally drift from execution. Furthermore, v4 declared the plan "immutable" but this was aspirational: a `[]Operation` is a slice of interfaces, and a Go interface can wrap a mutable pointer-backed struct, so the "freeze" was not actually enforced.

**The mechanism (in `pkg/plan`):**

type Plan struct {  
    ID        string        // a ULID, recorded in the operation history  
    Ops       \[\]Operation   // VALUE-SEALED after Seal(); see below  
    CreatedAt time.Time  
    sealed    bool  
}

// Seal deep-copies every concrete operation into a pointer-free VALUE form  
// that retains NO references into scanner buffers, SessionState, or caches.  
// After Seal() the plan cannot change even if the source state mutates.  
func (p \*Plan) Seal() \*Plan

* After discovery completes, the engine assembles the **complete** operation set (not the paged view) and calls `Seal()`, which deep-copies each concrete operation into a **value form that retains no pointers** into any live state (scanner buffers, session state, caches). Concrete operation types are deliberately designed as value types to make this possible.  
* The **gate confirms this exact sealed plan**, showing complete totals — total count, total reclaimable bytes, and per-kind reversibility — not the paged subset.  
* The **executor consumes only this sealed plan**. It cannot pick up extra work that the UI happened to be displaying. Structural drift between preview and execution is therefore impossible.  
* The `Plan.ID` is written to the operation history so a run is fully reconstructable.  
* This immutability is **enforced by test**: a test mutates scanner and session state after `Seal()` and asserts the plan is byte-identical (§15).

### **4.6 fd-anchored recursive deletion and firmlink awareness**

**The problem.** Go's `os.RemoveAll` walks a directory tree by path and re-resolves path components as it descends. It is **not safe against a concurrent directory-to-symlink swap**: if an attacker (or a coincidental rename) replaces a directory with a symlink mid-descent, `os.RemoveAll` will follow the symlink and can destroy targets *outside* the intended subtree. For a tool whose entire pitch is safety, deleting based on a path that can be redirected underfoot is unacceptable.

**The mechanism (in `pkg/safety/fsdelete`) — used only in obliterate mode:**

* Descend using `openat(parentfd, name, O_NOFOLLOW|O_DIRECTORY|O_CLOEXEC)`, **relative to directory file descriptors**, never re-resolving full paths. Because traversal is anchored to an open fd of the parent directory, a swap of a higher component cannot redirect the descent.  
* `fstatat` each entry with no-follow semantics; verify the entry's device \+ inode against the identity the plan recorded; reject any entry whose type has changed.  
* Remove entries with `unlinkat` (files) and `unlinkat(..., AT_REMOVEDIR)` (now-empty directories), in **post-order** (children before parents).  
* A swapped or replaced component fails the no-follow `openat` or the identity check and is **skipped and reported** — the deletion can never escape the intended subtree.  
* As an alternative single-shot path for non-fd-critical cases, the macOS `removefile(3)` C API with `REMOVEFILE_RECURSIVE` (which has its own hardened semantics) may be used; the fd-anchored walker is the default because it gives maximum control and reporting.

**Firmlink awareness (the correction).** macOS separates the read-only System volume from the writable Data volume and binds them with **firmlinks** — for example `/var` maps to `/private/var`, and `/Users` maps to `/System/Volumes/Data/Users`. Firmlinks are **not** symlinks, so `O_NOFOLLOW` does not reject them and the descent proceeds — but **crossing a firmlink changes the device number**, which would trip the device+inode identity check and cause Suns to wrongly abort a perfectly legitimate path. Therefore the identity layer (`pkg/safety/firmlink`) **detects OS-synthesized firmlink boundaries** (via `statfs`/`MNT_*` flags and a known-firmlink map) and **permits the device-number change across exactly those boundaries**, while still rejecting user-space symlink redirections. The firmlink allowance is constrained to the known OS map; it is not a general "ignore device changes" escape hatch.

**Trash-mode directory semantics (the clarification).** In trash mode, an entire directory subtree is moved **atomically at the approved root** through `trashItemAtURL:` (or the §4.4 pure-Go fallback). The OS (or the fallback) relocates the whole directory as a unit; the fd-anchored walker is **not** used in trash mode — it is obliterate-only. Identity between validation and trashing is preserved by re-stat-ing the root immediately before the move; per-child identity checks are unnecessary because the entire approved root moves together.

### **4.7 Identity-aware execution validation (tiered and privilege-correct)**

**The problem.** Between the moment a target is discovered (scan time) and the moment it is acted upon (execution time, after the operator confirms), the target's identity can change. A path can be renamed or replaced (a file swap), and a PID can be **recycled** by the OS to a completely different process. Validating only the *kind* of operation, or trusting a PID or a path that was recorded at scan time, is insufficient.

**The mechanism.** `ValidateAtExec` returns a typed `Identity` proving the target is still what was planned, and `Execute` refuses to act without a matching `Identity`. The validation is specific to each operation kind, and it is **tiered** for files (to avoid blocking on huge files) and **privilege-correct** for processes (to avoid an impossible privilege requirement):

* **FileDelete.** Re-`lstat` with no-follow semantics, then verify identity in a **tiered** way: files **above a size threshold** (for example 50 MB) are identified by **size \+ mtime \+ inode** (cheap, no full read), while **small high-risk files** (configuration files, dotfiles) are identified by a **full SHA-256 content hash**. All hashing is **backgrounded and cancellable**, so even a plan containing large files never blocks the execution pipeline. The deny-floor (§5.3) and firmlink-aware anchoring (§4.6) are also rechecked here.

* **ProcessKill — delegated to the privilege chokepoint (the correction).** Validating a process's identity means reading its **start (birth) time** and **executable path**, and ideally its code-signing team identifier. But on macOS, reading those attributes for a process owned by **root or another user** requires elevated privilege — and Suns's discovery layer is deliberately unprivileged (§6.4). An unprivileged validation would therefore *fail* on exactly the system daemons an administrator most needs to kill. The resolution: `ProcessKill` validation is **delegated to the privilege chokepoint**. The unprivileged TUI passes the chokepoint a tuple of `{PID, expected birth time, expected executable path}`; the chokepoint **validates and signals atomically under elevation** — confirming the PID still refers to the same process (defeating PID reuse) and only then sending `SIGTERM`/`SIGKILL`. Own-user processes can be validated unprivileged as a fast path.

* **ServiceUnload.** Match the launchd **domain \+ label \+ source plist path**; refuse if the label now resolves to a different plist or a different domain.

* **Restore.** Verify the trashed object against its recorded tiered identity (size+mtime+inode for large objects, content hash for small high-risk ones) before putting it back; a modified or swapped Trash entry is refused (§4.8).

* **ContainerPrune / RepoMaintenance.** Re-confirm daemon reachability / repository cleanliness immediately before acting.

### **4.8 Restore — honest, tiered, fail-loud**

`suns restore` reads the operation-history log and acts **only on records classified as Reversible `file_delete`**.

* **Identity verification.** Before restoring, the trashed object is verified against its recorded **tiered identity** (§4.7) — large objects by size+mtime+inode, small high-risk ones by content hash. A mismatch means the Trash entry was modified or swapped after deletion; the restore is **refused** and reported ("Trash entry was modified").  
* **Restoration uses Suns's own recorded original path**, not the OS "Put Back" facility. This is deliberate: it makes restore behave **identically** whether the file was trashed via the Cocoa API or via the pure-Go fallback (§4.4). OS Put-Back remains a nice-to-have for Finder integration but is never Suns's restore mechanism.  
* **Collision and edge handling:**  
  * *Original path is now occupied:* do not overwrite. Restore alongside as `name (restored <timestamp>).ext` and report.  
  * *Parent directory is gone but recreatable:* recreate it (with the original permissions if recorded) and restore.  
  * *Parent gone and not recreatable, or the target is not writable:* **fail loudly and report.** Suns does **not** silently relocate the file to an inert holding folder, because relocating a configuration file or application binary to `~/Library/Application Support/Suns/restored/` renders it useless to the OS — that is a second trash can masquerading as success. Relocation to a holding folder is offered **only** for items classified as plain user data, and **only** as an explicit operator choice.  
* **Irreversible records** (obliterated files, killed processes, flushed caches) are listed as unrecoverable and never attempted.

### **4.9 Configuration and session state (decoupled from Viper)**

Viper loads the configuration file **once at startup** into a plain Go struct `SessionState` guarded by a `sync.RWMutex`. Engines receive a `*SessionState` (or a read-only accessor) and call `state.ConfirmMode()` (taking a read-lock); they **never** call `viper.GetBool` at runtime. TUI toggles (`ctrl+d` for Deathstar, `ctrl+j` for Jarjar) take the write-lock and mutate the struct; no worker can observe a half-written value, eliminating the race that would otherwise panic. Viper is written back to disk **only** on an explicit "Save to config" action.

**State precedence (one-shot CLI invocations):** inline flags \> saved config file \> hardcoded safe defaults. For example `suns clean --deathstar --jarjar=false` overrides the config, which overrides the defaults of `confirm_mode=off, deletion_mode=trash`.

**State in the TUI:** the in-memory `SessionState` is seeded from the config at launch, toggled live, and is session-scoped unless explicitly saved.

**Scheduled runs** ignore both flags and interactive config and use locked settings (§12.20).

### **4.10 The operation-history log**

Located at `~/Library/Application Support/Suns/history.jsonl`, mode `0600`, append-only, crash-safe (JSON Lines — one self-contained JSON object per line). Each operation writes a **typed** record carrying the `Plan.ID`:

// FileDelete  
{"ts":"2026-06-09T14:32:08Z","plan":"01J...","op":"file\_delete","path":"/Users/x/Library/Caches/foo",  
 "size":12345,"fate":"trashed","trash\_path":"/Users/x/.Trash/foo","orig\_path":"/Users/x/Library/Caches/foo",  
 "identity":{"tier":"large","size":12345,"mtime":"...","inode":98765},"reversible":"reversible","cmd":"clean"}  
// ProcessKill  
{"ts":"...","plan":"01J...","op":"process\_kill","pid":941,"name":"Xcode","birth":"...","exec":"/Applications/Xcode.app/...",  
 "signal":"SIGKILL","reversible":"irreversible","cmd":"ps"}  
// DNSFlush  
{"ts":"...","plan":"01J...","op":"dns\_flush","reversible":"irreversible","cmd":"clean","status":"ok"}  
// RepoMaintenance  
{"ts":"...","plan":"01J...","op":"repo\_gc","repo":"/Users/x/dev/proj","aggressive":false,"prune":"2w","reversible":"recoverable"}  
// ReceiptForget (uninstaller)  
{"ts":"...","plan":"01J...","op":"receipt\_forget","pkgid":"com.vendor.app","reversible":"irreversible"}  
// Retained shared dependency (informational, not a deletion)  
{"ts":"...","plan":"01J...","op":"retained\_shared\_dependency","path":"/usr/local/lib/libfoo.dylib","claimed\_by":\["com.a","com.b"\]}

This typed, per-kind structure makes the reversibility claim **honest**: the log records *what kind* of action happened and *whether* it can be undone, instead of pretending everything is a restorable file. **It is an operational record, not a tamper-evident forensic trail** — there is no cryptographic chaining or signing in v1, and the program describes it accordingly. The canonical log is **full fidelity** (real absolute paths and identities) because restore and debugging depend on it; redaction is an **export-only** transform (§13.3).

---

## **5\. macOS Platform Reality**

The original blueprint did not mention macOS permissions at all, and this is the single biggest practical difference between a cleaner that works on the author's machine and one that works everywhere. This section makes the platform's realities first-class.

### **5.1 TCC / Full Disk Access (FDA)**

On Ventura and later, walking many of the directories a cleaner most wants to touch — `~/Library`, `~/Documents`, `~/Downloads`, `~/Desktop`, most of `~/Library/Application Support`, Mail, Messages, and more — requires the **host application** to have been granted **Full Disk Access** in *System Settings → Privacy & Security → Full Disk Access*. The "host application" is the terminal emulator running Suns (Terminal.app, iTerm2, Ghostty, Warp, …) or a signed Suns binary launched directly.

Suns handles this as a first-class state, not a crash:

1. **Early-warning probe (a hint, not a verdict).** At startup, Suns performs **one bounded, killable probe** of a single representative protected path purely to warn early. This is explicitly heuristic — TCC-denied surfaces are not perfectly uniform, so a single probe can give a false reassurance or a noisy warning. It is therefore treated as a hint only.  
2. **Authoritative detection during the real scan.** The walker treats a per-path `EPERM` / "operation not permitted" as a **skip-with-reason**, aggregates these, and infers "limited mode" when protected categories return systematic denials. A non-entitled command-line binary receives `EPERM` here; it does **not** trigger the entitlement-app TCC modal (so the v3 worry about a blocking hidden prompt does not apply to a plain CLI). The proposed `sandbox-exec` profiling approach is rejected as vaguer and less reliable than this.  
3. **Guidance, not failure.** When limited mode is detected, Suns shows a friendly onboarding panel explaining why FDA is needed and offering to open the exact settings pane via the `x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles` URL.  
4. **Graceful degradation.** Without FDA, Suns scans the paths it can, badges protected categories "🔒 needs Full Disk Access," and **never presents a partial scan as if it were complete.**  
5. **Honest framing of signing.** Code signing and notarization aid distribution and trust; they do **not** grant automatic protected-path access. TCC permissions attach to specific executable identities and contexts, so the user must still grant FDA to the terminal or to Suns explicitly.

### **5.2 Apple Silicon versus Intel**

* **Thermals and fans.** Apple Silicon uses IOKit (`IOHIDEventSystemClient`) or `powermetrics`; Intel uses the legacy System Management Controller (SMC). The sensor keys **differ across the M1, M2, M3, and M4 generations**, so a hardcoded Cgo bridge would break on new hardware. Suns ships an **embedded** board-ID → sensor-key manifest (`assets/sensors.json`), updated with each release. It is **not** fetched remotely (that would add a network and update-trust surface to a security tool). An unknown board ID degrades to "N/A" rather than crashing, and fanless Macs (the MacBook Air) correctly report **no fans** as "N/A" rather than showing 0 RPM as if the hardware were broken.  
* **Universal binary.** Suns builds for both `arm64` and `amd64` and is lipo'd into a single universal binary so one download runs everywhere.

### **5.3 The never-touch deny floor**

`pkg/safety` ships a versioned manifest of paths that **nothing in the program may ever delete**, enforced at **plan time, execution time, and every directory descent**:

* `/System`, `/usr` (except `/usr/local`), `/bin`, `/sbin`, all SIP-protected paths, `/Library/Apple`, anything on a System Volume, **the user's home root itself** (only specific subpaths are ever targeted, never `~` directly), and iCloud-synced containers by default.  
* **No flag, no mode, no config value, and no scheduled job can bypass the floor.** `confirm_mode=on` and `deletion_mode=obliterate` do not override it.

Alongside the deny floor are the **safe-cache allowlist** (an explicit, named set of directories known to be safe to clear, each tagged with a rebuild-cost level — never a blanket `~/Library/Caches` sweep, which can log the user out of apps or trigger multi-gigabyte iCloud re-syncs) and the **user exclusion file** (`~/Library/Application Support/Suns/exclude.yaml`, a list of path globs the operator pins as off-limits, honored by every engine).

### **5.4 Code signing & distribution**

Suns is code-signed with a Developer ID and notarized, so that protected-path reads are smooth and the binary is trustworthy and distributable. It ships via a signed `.pkg` and a Homebrew tap. The release pipeline (universal binary build \+ the required Cgo paths \+ codesign \+ notarize \+ Homebrew tap) is non-trivial and is treated as its own first-class roadmap phase (§14), not an afterthought.

---

## **6\. The Privilege Model**

Several capabilities require root: flushing DNS, querying the unified log for auth events, some thermal reads, installing the scheduled daemon, and validating-then-killing processes owned by root or other users. Suns treats privilege as a security boundary, not a convenience, and arrives at its model by elimination of two earlier, flawed approaches.

**Why not the obvious approaches.** v1's implicit approach (`osascript … with administrator privileges`) pops the GUI SecurityAgent dialog, which steals focus from the terminal, loses keystrokes if the user keeps typing, returns a cryptic "User canceled" error on escape, and does not cache the sudo ticket (so it re-prompts for every action). v3's primary approach (a signed `SMAppService` \+ XPC helper) is impractical from Go — there are no native XPC bindings, and bridging Go's garbage collector to XPC's manual reference counting across goroutines is a volatile Cgo minefield — and helper approval fits an app-bundle/installer distribution far better than a bare Homebrew CLI. v4's fallback idea of a "persistent elevated runner" was dropped because a long-lived root process with a command channel is itself an unspecified ambient-root control surface that a safety-focused tool should not spawn.

### **6.1 The model: a centralized, per-workflow `sudo` chokepoint**

All privileged execution funnels through **one** elevation chokepoint in `pkg/privilege`, never scattered across engines. It runs commands through the hardened `syscmd` layer (§6.3) and works as follows:

* **Acquire a ticket per workflow.** When a workflow needs root, the chokepoint runs `sudo -v` via Bubble Tea's `tea.Exec` / `ExecProcess`. This is essential and correct: because Bubble Tea owns the terminal in raw mode, you cannot simply attach `sudo` to stdin mid-session; `tea.Exec` **releases the terminal**, runs `sudo -v` inline with real stdio so the password prompt renders directly in the terminal, then restores the TUI.  
* **TTY-ticket correctness.** macOS `sudo` uses `tty_tickets`, meaning a ticket is bound to a specific controlling TTY. A privileged background subprocess (such as the long-lived `powermetrics` stream, or a privileged validate-and-kill) spawned with different process attributes would **not** inherit a ticket bound to the interactive TTY and would silently fail to elevate. Suns handles this explicitly: any privileged subprocess is spawned **as a child of the same elevated session with the correct controlling-TTY file descriptor attached**, so it inherits the ticket.  
* **No persistent root daemon.** Suns keeps no long-lived root process. The accepted trade-off is that a long-running privileged background stream may need to **re-prompt** when its ticket expires; Suns prefers a clear re-prompt over keeping an ambient-root control surface alive.  
* **Defined failure states.** *Canceled prompt:* the dependent operation(s) are skipped, marked `skipped:no-privilege` in the history, and the workflow continues for any non-privileged operations. *Ticket expiry mid-workflow:* the chokepoint detects the auth failure, re-prompts once via `tea.Exec`, and aborts the privileged remainder cleanly on a second failure. *Partial privileged batch:* each privileged operation is independently validated and recorded, so a mid-batch failure leaves a precise history of what did and did not run — no all-or-nothing ambiguity.  
* Every privileged action is labeled in the UI with a 🔐 badge and a one-line explanation of *why* root is needed.

### **6.2 Optional / future: an app-bundle helper**

If a future app-bundle distribution of Suns is pursued, a signed `SMAppService` helper with XPC could be added to avoid repeated prompts. It would slot in **behind the same chokepoint** without changing any caller. This is explicitly **out of scope for the CLI / Homebrew v1**, and nothing in the design depends on it.

### **6.3 Hardened command execution (`syscmd`)**

Every external command — privileged or not — goes through `pkg/syscmd`, which enforces, for a security-sensitive utility:

* **Absolute, pinned executable paths** (`/usr/bin/lsof`, `/usr/sbin/nettop`, `/usr/bin/pkgutil`, …) — no `PATH` lookup, so no `$PATH` hijack is possible.  
* **Scrubbed environment** with an explicit minimal set, and **`LC_ALL=C`** so command output is locale-deterministic and parseable.  
* **No shell.** Commands are run as `exec.Command(bin, args...)` with arguments passed as a slice; arguments are never interpolated into `sh -c`, which eliminates shell-injection.  
* **A compiled-in command allowlist** — any attempt to run a command not on the list is a programming error caught in tests.  
* **Bounded output** — stdout/stderr are capped to a maximum and truncated with a flag, so a runaway tool cannot exhaust memory.  
* **A context timeout on every call**, and a per-command **parsing contract** with version detection and graceful degradation (§13.1).

### **6.4 The privilege boundary (explicit)**

* **All scanning and target discovery is unprivileged.** Suns does not self-escalate in order to read files; the operator grants Full Disk Access to the terminal or to Suns for protected reads.  
* **The elevation chokepoint executes only narrow, allowlisted root *actions*** — DNS flush, the auth-log `log show` query, `pkgutil --forget` and `pkgutil --file-info`, daemon installation, and the privileged validate-and-kill for `ProcessKill`. It **never** performs broad protected-path *reads* on behalf of the scanner.  
* If a future helper exists (§6.2), this same boundary holds: the helper is a narrow allowlisted action runner, never a broad filesystem reader.

---

## **7\. Telemetry Architecture**

The telemetry subsystem powers `suns monitor` and the `get-coffee` dashboard. Its design is the result of correcting a fundamental performance mistake and then making the streams robust over long sessions.

### **7.1 The core mistake and its correction**

The naive design polls heavy tools like `nettop` and `powermetrics` on a one-second ticker by spawning and tearing them down each time. This is **computationally destructive**: both are heavyweight, buffered, interactive tools, and continuously spawning them spikes CPU and **artificially inflates the very readings Suns is trying to measure**. The correct approach is to launch each heavy source **once** as a long-lived streaming process and decode its output continuously.

### **7.2 Long-lived streams and a single snapshot**

A single `telemetry.Poller` builds one `*SystemSnapshot` per cadence and publishes it via `atomic.Pointer` (the same lock-free mechanism as §3.2); every widget reads the same snapshot, so there is no per-widget polling and no redundant syscalls. Sources:

* **In-process (cheap, \~1 Hz):** `gopsutil` for CPU (aggregate and per-core), memory, disk usage, disk I/O counters, host info, and process enumeration. No subprocess.  
* **`powermetrics` (root, streamed):** a single `exec.CommandContext` launched once with an interval flag and plist output, connected via a stdout pipe and decoded continuously through the tokenizer (§7.3). Supplies GPU utilization, package/system power, and (on Apple Silicon) thermals. Root is acquired via the §6 chokepoint.  
* **Network (sysctl preferred, `nettop` fallback):** interface-level throughput is read from Darwin `sysctl` (`net.inet.tcp.pcblist` / interface counters) via `golang.org/x/sys/unix`, which is reliable. Per-process attribution, where attempted, uses a long-lived `nettop -P -l 0` stream — but this is **experimental** (§7.5).  
* **IOKit thermals (Apple Silicon, bounded Cgo):** sensor reads via `IOHIDEventSystemClient`, each under a per-call timeout in a bounded goroutine (§4.4), supervised.

Heavy sources update on a **slower internal cadence** (every 2–3 seconds) than the cheap gopsutil sources (1 second); the poller merges them into one snapshot. Each metric has fixed-size **ring buffers** (around 60 samples) feeding the sparklines — bounded memory, cheap.

### **7.3 The `powermetrics` stream tokenizer (with a read-deadline)**

`powermetrics -f plist` emits a **concatenated** sequence of discrete plist documents; `howett.net/plist` expects a single complete document and fails at the first concatenation boundary. The tokenizer therefore scans the byte stream for document delimiters (the null byte / `<?xml` … `</plist>` boundaries), buffers **one complete block at a time**, and feeds each to the decoder individually — the decoder never sees a partial or multi-document buffer.

**Critically, the tokenizer sets a read-deadline.** Under extreme system load (thermal throttling, heavy swap), the `powermetrics` daemon can stall mid-flush, leaving a partial document in the pipe with no closing `</plist>` and no EOF — a state that is neither "dead" nor obviously "wedged," so a naive watchdog would never fire and the tokenizer would hang forever. With a read-deadline (for example, the poll-interval boundary, \~3 seconds), if a complete document does not arrive in time the tokenizer **flushes its buffer, marks the source `stale`, and signals the supervisor to restart the subprocess.** "Stalled" is thus a first-class supervised state, distinct from "EOF/dead."

### **7.4 Supervised streams (never show dead data as live)**

Each telemetry source is a **supervised service**, not a fire-and-forget pipe:

* **Per-source health state:** `live`, `stale`, `stalled`, `restarting`, or `unavailable`.  
* **Per-field last-sample timestamp** carried in the snapshot. The dashboard renders a **staleness badge** on any tile whose data is older than a threshold (for example, CPU shows 🟢 fresh while THERMAL shows ⚠ stale 8s), so the operator is never misled into reading frozen data as current.  
* **Watchdog with exponential backoff:** if a stream wedges or hits EOF, the supervisor restarts it with backoff and marks the source `restarting`; persistent failure becomes `unavailable` with a reason — never a frozen-but-live-looking tile.  
* **Parser resync:** on a malformed block, the tokenizer skips to the next document boundary rather than desynchronizing the whole stream.  
* This supervision covers `powermetrics`, `nettop`, **and the bounded IOKit thermal reads** uniformly.

### **7.5 `nettop` honesty (experimental in early releases)**

Per-process network attribution on Darwin is structurally the most fragile feature in the entire design, even with the parsing-contract layer (§13.1). Suns therefore ships it **flagged experimental / optional in early releases.** Interface-level totals (reliable) are always shown and are the core promise; per-process attribution is labeled approximate and **degrades cleanly to "unavailable"** when parsing drifts, rather than presenting wrong numbers confidently.

---

## **8\. Project Layout**

suns/  
├── cmd/  
│   └── suns/  
│       └── main.go               \# Entry point; Cobra root; session bootstrap;  
│                                  \#   reads config into SessionState; launches CLI or TUI  
├── internal/  
│   ├── cli/                      \# Cobra command definitions:  
│   │                             \#   clean, nuke, dedup, scan, monitor, get-coffee,  
│   │                             \#   net, audit, maintain, schedule, restore, config, doctor  
│   ├── config/                   \# Viper load → RWMutex-guarded SessionState (§4.9)  
│   ├── tui/  
│   │   ├── app.go                \# Root Bubble Tea model; router between views  
│   │   ├── views/                \# menu, clean, nuke, dashboard, audit, net, scan, restore …  
│   │   ├── components/           \# gauge, sparkline, stattile, table, modal,  
│   │   │                         \#   statusbar, toast, spinner, help, onboarding  
│   │   └── theme/                \# lipgloss palettes; mode-based accent recoloring;  
│   │                             \#   Apple\_Terminal sparkline fallback  
│   ├── scanner/                  \# Concurrent WalkDir discovery → double-buffered ScanSnapshot;  
│   │                             \#   category manifests; matching  
│   ├── uninstaller/              \# .app/bundle-ID tracing; pkgutil \--files harvest;  
│   │                             \#   pkgutil \--file-info shared-dependency guard; → ops; → forget  
│   ├── dedup/                    \# APFS-conservative 3-pass; xattr normalization; keeper heuristic  
│   ├── optimizer/                \# DNS flush, mdutil, and similar action wrappers  
│   ├── telemetry/                \# Supervised poller; powermetrics tokenizer; sysctl; bounded IOKit  
│   ├── audit/                    \# SIP/Gatekeeper/FileVault posture; unified-log auth analysis  
│   ├── net/                      \# socket map, port audit, LAN scan, bandwidth  
│   └── scheduler/                \# launchd job authoring; failure semantics  
├── pkg/  
│   ├── operation/                \# Operation interface \+ concrete VALUE-type op kinds (§4.2)  
│   ├── plan/                     \# Plan \+ Seal() value-copy (§4.5)  
│   ├── safety/  
│   │   ├── gate.go               \# Confirms a frozen Plan; renders per-kind preview  
│   │   ├── floor.go              \# Never-touch deny list; enforced plan/exec/descent  
│   │   ├── revalidate.go         \# Identity-aware ValidateAtExec; tiered identity (§4.7)  
│   │   ├── fsdelete.go           \# fd-anchored recursive delete (§4.6)  
│   │   └── firmlink.go           \# Firmlink-aware identity handling (§4.6)  
│   ├── trash/                    \# Bounded Cgo NSFileManager \+ killable volume probe \+  
│   │                             \#   semaphore \+ circuit breaker \+ pure-Go fallback (§4.4)  
│   ├── history/                  \# JSONL operation history (0600); export-redaction (§13.3)  
│   ├── privilege/                \# Centralized per-workflow sudo chokepoint (§6); no persistent runner  
│   ├── syscmd/                   \# Hardened exec (pinned paths, LC\_ALL=C, allowlist) \+ parsing contracts  
│   └── plist/                    \# howett.net/plist \+ plutil fallback \+ powermetrics stream tokenizer  
├── assets/                       \# //go:embed: ASCII logo, OUI vendor DB, safe-cache manifest, sensors.json  
├── go.mod  
└── go.sum

The `assets/` directory is embedded into the binary with `//go:embed`, so the program is fully self-contained: the ASCII sun logo, the MAC-vendor OUI lookup table for the LAN scanner, the safe-cache allowlist manifest, and the board-ID → sensor-key map all travel inside the single binary.

---

## **9\. Command Taxonomy**

The organizing principle is the separation of **destructive** from **read-only** commands. Only destructive commands route through the safety gate and emit typed operations into a frozen plan; read-only commands never prompt and never delete.

### **9.1 Destructive commands (gated; emit typed Operations into a sealed Plan)**

| Command | Purpose | Operation kinds emitted |
| ----- | ----- | ----- |
| `suns clean` | Curated system & dev cleanup | `FileDelete` (safe caches, logs, dev-caches, empty dirs, broken symlinks), `CacheReset` (DNS flush), `ContainerPrune` (Docker), `ServiceUnload` (orphaned launch agents) |
| `suns nuke <app>` | Precision uninstaller | `FileDelete` (bundle \+ remnants \+ shared-dependency-guarded `.pkg` payload), `ServiceUnload`, `ReceiptForget` |
| `suns dedup [path]` | Duplicate file removal | `FileDelete` (APFS-conservative) |
| `suns maintain` | Repository maintenance | `RepoMaintenance` (git gc, de-fanged) |
| `suns lang-strip <app>` | Localization stripping (quarantined) | `FileDelete` |
| `suns schedule` | Author scheduled cleanups | (authors a constrained launchd job) |
| `suns restore` | Undo from operation history | (reverses Reversible `FileDelete` records, identity-checked) |
| `suns ps` (kill action) | Kill a selected process | `ProcessKill` (privilege-delegated, identity-checked) |

### **9.2 Read-only commands (no gate, no prompts)**

| Command | Purpose |
| ----- | ----- |
| `suns scan` / `suns analyze` | Disk-usage visualizer; broken-symlink audit; repo report |
| `suns monitor` | One-shot system stats (CPU, memory pressure, thermals) |
| `suns get-coffee` | The full live telemetry dashboard (§11) |
| `suns net` | Socket map, open-port audit, LAN scan, bandwidth (experimental per-PID) |
| `suns audit` / `suns secure` | SIP/Gatekeeper/FileVault posture; auth-log analysis |
| `suns ps` (listing) | Process inspection; runaway/zombie flagging |

### **9.3 Meta commands**

| Command | Purpose |
| ----- | ----- |
| `suns config (get|set|show)` | Read/write persisted configuration; `show` prints the effective merged config and the source of each value |
| `suns doctor` | Environment, permission, tool-version, and capability self-check |
| `suns version` | Version and build info |
| `suns` (no subcommand) | Launches the interactive TUI |

### **9.4 Global flags & scriptability**

* `--dry-run` (destructive commands): prints the **frozen plan** as text or JSON and exits 0 without acting.  
* `--yes` (destructive commands): bypasses the confirmation gate (equivalent to `confirm_mode=on`).  
* `--deathstar` / `--jarjar=<trash|obliterate>`: per-invocation overrides of the two safety axes.  
* `--json` (read-only commands): structured output for scripting.  
* `--verbose` / `--quiet`: application-log verbosity.  
* `--reduce-motion`: disables TUI animation.

---

## **10\. The TUI: Design System, UX & Aesthetics**

A TUI is not an excuse for an ugly or confusing interface. Suns invests in a genuine design system; the quality bar is `lazygit`, `k9s`, and `btop`. Everything below is a buildable component or rule.

### **10.1 Visual identity**

The name and tagline ("Suns / Burn It All") drive a **solar, fire-and-plasma** aesthetic: a warm core palette with a cool "safe" counterpoint. The splash and the menu header render an embedded ASCII / figlet **sun logo**, animated with a subtle harmonica-driven shimmer that cycles color along the rays. The identity is distinctly Suns rather than a generic boxed-TUI look.

### **10.2 The adaptive color system and mode recoloring**

Two coordinated lipgloss palettes exist, and the **active confirmation mode recolors the entire chrome** (borders, accents, the status bar) so the operator's safety posture is impossible to miss.

| Token | Safe mode (`confirm_mode` off) | Armed mode (`confirm_mode` on) |
| ----- | ----- | ----- |
| `accent` | Cool plasma blue `#4FC3F7` | Aggressive ember red `#FF3B30` |
| `accentDim` | `#2B7A99` | `#A11` |
| `success` | `#34C759` | `#34C759` |
| `warning` | `#FFD60A` | `#FF9F0A` |
| `danger` | `#FF453A` | `#FF453A` |
| `text` / `muted` | `#E6E6E6` / `#8A8A8A` | same |
| panel surfaces | transparent / `#1c1c1e` | transparent / `#2a1112` |

Rules: use `lipgloss.AdaptiveColor` so light-terminal users get legible variants; respect the `NO_COLOR` environment variable and degrade to a monochrome-plus-symbols theme; detect truecolor versus 256-color versus 16-color via the terminal environment and pick the richest available. The **Jarjar (deletion-mode) indicator is independent of the accent**: a small chip in the status bar shows `🗑 TRASH` (green) versus `🔥 OBLITERATE` (red).

### **10.3 The layout grammar**

Every screen uses a consistent three-zone frame composed with `lipgloss.JoinVertical` / `JoinHorizontal`:

┌─ Suns ─────────────────────────── ◐ deathstar:OFF   🗑 jarjar:TRASH ──┐  ← Header / mode bar  
│                                                                        │  
│   \[ Active view: menu · table · dashboard · preview modal · onboarding \]│  ← Body (flex)  
│                                                                        │  
├────────────────────────────────────────────────────────────────────────┤  
│ ↑↓ move · enter select · ctrl+d deathstar · ctrl+j jarjar · ? help · q  │  ← Footer / keybind hints  
└────────────────────────────────────────────────────────────────────────┘

It is **responsive**: on a `tea.WindowSizeMsg`, panel widths are recomputed, the dashboard grid reflows (4-up → 2-up → 1-up as width shrinks), and long paths are **middle-ellipsized** (`/Users/…/DerivedData`) rather than overflowing. The **focus model**: Tab cycles focusable regions; the focused panel gets a bright border, the others a dim border.

### **10.4 The component library (`internal/tui/components`)**

* **Gauge** — a horizontal or radial bar with a gradient fill, used for CPU, memory, and disk. Its value **eases** to the target via harmonica spring physics rather than jumping, giving smooth, readable motion (honoring `--reduce-motion`).  
* **Sparkline / line chart** — a braille-based rolling-history chart (via ntcharts) fed by fixed-size ring buffers, with an **automatic block-element fallback when `TERM_PROGRAM == Apple_Terminal`** (Braille line-height bug).  
* **Stat tile** — the atomic dashboard unit: a label, a large value, a trend arrow (▲▼), a mini sparkline, and a **staleness badge** (§7.4). The dashboard grid is assembled from these.  
* **Smart table** — a sortable, scrollable table (on `bubbles/table`) with size formatting and category coloring, **fed by the double-buffered snapshot** so re-sorting hundreds of thousands of rows never stalls the scanner or blocks rendering. Used in clean, dedup, nuke, and process views.  
* **Frozen-plan confirmation modal** — the visual embodiment of the safety gate: a centered overlay that dims the background and renders the **sealed plan** (a file table, a process table, or a one-liner per operation kind) with complete totals and a 🟢/🟡/🔴 reversibility badge per group. `enter` confirms, `esc` cancels; skipped entirely when `confirm_mode` is on.  
* **Toast / notification** — transient bottom-corner messages ("Trashed 1,204 files · 3.2 GB reclaimed") for non-blocking feedback.  
* **Spinner \+ progress** — for scans: a spinner plus a progress readout showing the current path (middle-ellipsized) and reclaimable-so-far, driven from the engine's shared aggregate at \~10 Hz, decoupled from worker throughput.  
* **Onboarding / FDA panel** — shown when limited-mode is detected; explains Full Disk Access and opens the settings pane (§5.1, §10.7).  
* **Help overlay** — a `?`\-triggered overlay (on `bubbles/help` \+ a `key.Map`) listing all keybindings.

### **10.5 Motion & feel**

Motion is purposeful only: gauge easing, a brief "burn" animation when files are deleted, and smooth view transitions, each completing within roughly 250 ms and all honoring a `--reduce-motion` flag. There is no gratuitous animation. The render budget coalesces updates so the UI redraws at roughly 10–30 Hz even under heavy event load — stated as a goal, not a guarantee (§13.6).

### **10.6 Accessibility & ergonomics**

Full keyboard control is primary; mouse support (clickable tiles and menu items) is optional, via bubblezone. A discoverable help overlay (`?`) lists every keybinding. The interface is **colorblind-safe: it never relies on color alone** — every color signal is paired with an icon or a word (the TRASH/OBLITERATE chips, the 🔒 lock badges, the 🟢🟡🔴 reversibility badges, the ▲▼ trend arrows, the ⚠ staleness markers).

### **10.7 In-UI honesty about scope**

Features that are best-effort rather than complete **state their bounded scope in the UI**. The uninstaller, for instance, never says "complete uninstall"; it shows what it will and will not touch. This prevents the interface wording from reintroducing an overclaim the engine deliberately avoids.

---

## **11\. The `get-coffee` Live Telemetry Dashboard**

`suns get-coffee` starts a continuous, real-time monitoring session — fire it up, walk away to get a coffee, and the machine's vitals are laid out as live widgets docked along the top and bottom of the view, with a tabbed detail panel in the middle. It is **entirely read-only** (and therefore needs no Full Disk Access, making it the ideal first-run showcase), with one exception: the process-kill affordance, which routes through the safety gate as a privilege-delegated, identity-checked `ProcessKill` like any destructive action.

### **11.1 Layout**

┌─ get-coffee ─────────── ☕ uptime 4h12m · load 2.1 1.8 1.6 · 14:32:08 ─────────┐  
│ ┌─ CPU ───┐ ┌─ MEMORY ─┐ ┌─ GPU ──┐ ┌─ THERMAL ⚠┐ ┌─ BATTERY ┐ ┌─ NET ──┐    │  ← TOP widget rail  
│ │ 38% ▂▄▆█│ │ press: 🟢 │ │  22%   │ │ 64° stale │ │ 87% ⚡    │ │↓4.2MB  │    │  (⚠ \= stale badge)  
│ │ ▂▄▆█▅▃▂▁│ │ swap 0.4G │ │ ▁▂▃▂▁  │ │ fan 2400  │ │ 22.1W    │ │↑0.3MB  │    │  
│ └─────────┘ └──────────┘ └────────┘ └───────────┘ └──────────┘ └────────┘    │  
│                                                                               │  
│   \[ Main panel — tabbed detail: Top Processes / Network / Disk / Temps \]      │  
│   PID    NAME            CPU%   MEM     ↓/s     ↑/s    STATE                   │  
│   941    Xcode           120%   6.4G    0.1M    0.0M   running                 │  
│   …                                                                           │  
│                                                                               │  
│ ┌─ DISK I/O ──────────┐ ┌─ DISK SPACE ───────┐ ┌─ POWER / SENSORS ────────┐  │  ← BOTTOM widget rail  
│ │ R 12MB/s  W 4MB/s   │ │ ▓▓▓▓▓▓▓░░ 312/500GB │ │ pkg 18W · sys 24W        │  │  
│ │ ▃▅▂▁▄▆▃▂           │ │ 62% used            │ │ throttle: none           │  │  
│ └─────────────────────┘ └─────────────────────┘ └──────────────────────────┘  │  
├─────────────────────────────────────────────────────────────────────────────┤  
│ tab switch · k kill (gated) · p pause · \+/- poll · s sort · e elevate · q quit │  
└─────────────────────────────────────────────────────────────────────────────┘

The widget rails reflow responsively (4-up → 2-up → stacked), and which widgets dock top versus bottom is configurable.

### **11.2 The nine observables**

| \# | Widget | What it shows | Data source |
| ----- | ----- | ----- | ----- |
| 1 | **CPU** | Aggregate % \+ per-core mini-bars \+ 1/5/15-minute load averages \+ rolling sparkline | `gopsutil` cpu/load |
| 2 | **Memory pressure** | Wired / App / Compressed / Swap breakdown \+ the macOS pressure state (🟢🟡🔴) \+ swap-in/out rate sparkline | `vm_stat`, `sysctl`, `gopsutil` mem |
| 3 | **Network** | Live ↓/↑ per interface (reliable, core) \+ best-effort top-talker process (experimental) \+ rolling chart | `sysctl` (`x/sys/unix`); `nettop` for per-PID |
| 4 | **GPU** | Utilization % and, where available, VRAM, with a sparkline | `powermetrics` stream |
| 5 | **Thermal & fan** | CPU/GPU temperatures, fan RPM (or "N/A" on fanless Macs), throttle flag; carries a staleness badge | IOKit / `powermetrics` (Apple Silicon), SMC (Intel) |
| 6 | **Battery & power** | Charge %, charging/discharging state, time remaining, **live wattage draw**, cycle count, battery health | `pmset`, IOKit `AppleSmartBattery` |
| 7 | **Disk I/O** | Read/write bytes-per-second and IOPS, with a rolling chart | `gopsutil` disk IO counters |
| 8 | **Disk space** | Used/free per mounted volume, a percentage bar, and a low-space alert color | `gopsutil` disk usage |
| 9 | **Top processes** | A live-sorted table by CPU / memory / network, with the gated kill action | `gopsutil` process |

A header strip additionally shows uptime, load, and system power, giving a complete vitals picture — compute, memory, graphics, thermals, power, storage, and I/O — at a glance.

### **11.3 Controls & mechanics**

* **One snapshot, many widgets.** A single `telemetry.Poller` ticks (default 1 second, adjustable live with `+`/`-`), builds **one** `SystemSnapshot`, and publishes it via `atomic.Pointer`; every widget reads the same snapshot. No per-widget polling, no redundant syscalls.  
* **Staleness honesty.** Every tile carries a freshness timestamp and shows a stale/unavailable badge (§7.4) rather than presenting frozen data as live. Heavy sources tick more slowly than cheap ones; the badges make the difference visible.  
* **Pause / resume** (`p`) freezes the snapshot for inspection.  
* **Kill** (`k`) on the selected process is the one destructive affordance and routes through the gate with privilege-delegated identity validation (lists, confirms, SIGTERM → SIGKILL).  
* **Elevate** (`e`) acquires root via the §6 chokepoint for thermals; widgets needing root but lacking it show "🔐 press `e` to elevate" rather than a blank tile.  
* **No Full Disk Access required** — read-only telemetry works out of the box, before any permission onboarding.

---

## **12\. Feature Specifications**

This section details every feature engine at implementation depth. Each entry states the command(s) it backs, whether it is destructive (and thus gated) or read-only, its operation kind and reversibility, the privilege it needs, and exactly how it works — including the corrections that distinguish the final design.

### **12.1 Hash-Based Duplicate Immolator**

**Command:** `suns dedup [path]` · **Destructive · gated** · `FileDelete` · 🟢 trash / 🔴 obliterate. **What it does:** Finds and removes genuine duplicate files. **How it works:** Three passes for efficiency: (1) group candidates by exact byte size; (2) for each same-size group, hash only the **first 4 KB** as a cheap discriminator; (3) compute a full SHA-256 only on the survivors of pass 2\. Identity for the eventual delete is **tiered** (§4.7). **APFS correctness (the key correction):** APFS exposes block sharing via reference counts, and this does **not** identify clone lineage — you cannot prove that file A is a clone of file B, only that some blocks are shared. Therefore Suns does **not** exclude shared-block files from dedup (doing so would wrongly skip real duplicates that merely share blocks with unrelated objects such as Time Machine snapshots); instead it detects sharing only to **estimate reclaimable space conservatively** and to **flag** that the space actually freed for clone-participating files may be lower than the nominal size. **Hardlinks** (files sharing an inode) are the same file and are never offered as deletable duplicates. **Bundles** (`.app`, `.rtfd`, `.photoslibrary`) are directories that present as documents and are treated atomically — interior files are never deduped. **Xattr normalization policy:** the comparison is over data forks, **ignoring cosmetic extended attributes** (quarantine flag, Finder tags, where-from) so that meaningful duplicates are not rejected over harmless metadata; a note is surfaced when ignored xattrs differ. **Keeper heuristic:** the recommended file to keep is chosen by preferring a non-cache, non-trash, user-document location and deprioritizing copies under `Caches`/`Downloads`/temp; it never auto-selects across different bundles, and the operator reviews and adjusts the selection before the gate fires.

### **12.2 Sunburst / Disk-Usage Visualizer**

**Command:** `suns scan` / `suns analyze` · **Read-only.** **What it does:** Shows where disk space is going, navigably. **How it works:** A concurrent recursive size aggregation feeds an **ncdu-style navigable, sorted-bar drill-down** — arrow keys descend into and ascend out of directories, entries sorted by size, rendered from the double-buffered snapshot. This is deliberately chosen over a literal block-character treemap, which reads poorly in a terminal; an experimental treemap mode may exist as a secondary view. This command also hosts the read-only half of the broken-symlink auditor and a git-repo size report.

### **12.3 Orphaned Launch Agent Purge**

**Command:** part of `suns clean` · **Destructive · gated** · `ServiceUnload` \+ `FileDelete` · 🟡 · best-effort. **What it does:** Removes launch agents and daemons whose target executables no longer exist. **How it works:** Scans `~/Library/LaunchAgents`, `/Library/LaunchAgents`, and `/Library/LaunchDaemons`, parsing each plist in a **binary-safe** way (`howett.net/plist`) for its `Program` / `ProgramArguments`. Before flagging an entry as orphaned, it **resolves wrappers, shell scripts, relative program paths, and environment-launchers** so it does not false-positive on indirection; it **skips Apple-managed and app-regenerated** jobs; and it requires the *resolved* executable to be genuinely absent. It runs `launchctl bootout` (the `ServiceUnload` operation) to unload a loaded-but-dead service **before** removing the plist file. Identity is checked at execution by domain \+ label \+ source plist path. **Explicitly bounded:** it handles common cases and **documents** that launchd domains, disabled states, and update-time regeneration can race with bootout-then-remove; it reports rather than guarantees.

### **12.4 Language Localization Stripper**

**Command:** `suns lang-strip <app>` · **Destructive · gated** · `FileDelete` · 🔴 · **QUARANTINED.** **What it does:** Removes unused `.lproj` language resources from app bundles to reclaim space. **How it works:** This is the **most dangerous feature in the product** and is treated accordingly. Deleting `.lproj` folders from inside a `.app` bundle **modifies a signed application bundle, which invalidates its code signature**; on modern macOS with hardened runtime and notarization, this can make the app refuse to launch ("damaged — move to Trash"), trip Gatekeeper, and break Sparkle/delta auto-updates. It is therefore **opt-in, per-app, behind the strongest warning in the application**, never part of bulk `clean`, and it **re-runs `codesign --verify` afterward** and warns loudly if the bundle is now broken. The recommended default is to leave bundles intact. It ships last (§14).

### **12.5 Per-App Socket Mapper**

**Command:** part of `suns net` · **Read-only.** **What it does:** Maps which application owns which live network connection. **How it works:** Parses `lsof -i -n -P` (via the hardened executor, pinned path, `LC_ALL=C`) into a PID → application → local/remote address and port table, with asynchronous, cached reverse-DNS on the remote addresses. It highlights unexpected outbound connections.

### **12.6 Network Analyser (LAN scan)**

**Command:** `suns net scan` · **Read-only.** **What it does:** Discovers devices on the local network. **How it works:** Scoped to what is actually obtainable — IP address, MAC address (from the ARP table), vendor (via the **embedded OUI lookup table**), and hostname (reverse DNS plus mDNS/Bonjour), with an optional active port probe. It **warns before active scanning** on networks the operator may not own (corporate or public Wi-Fi), where active scanning is inappropriate. The original "every single thing about every device" framing is impossible and is deliberately not promised.

### **12.7 Live Bandwidth Hog Detector**

**Command:** part of `suns get-coffee` / `suns net` · **Read-only · EXPERIMENTAL.** **What it does:** Shows which processes are using the most network throughput, live. **How it works:** Computes per-process transmit/receive deltas over time. **Correction:** gopsutil does not reliably provide per-process network counters on Darwin, so this is implemented by parsing a long-lived `nettop -P -l 0` stream, with interface-level totals from `sysctl` as the reliable fallback. Because per-process attribution on Darwin is structurally fragile, this feature is **flagged experimental** in early releases and degrades cleanly to "unavailable" rather than showing wrong numbers.

### **12.8 Runaway / Zombie Process Inspector**

**Command:** `suns ps` / part of `suns get-coffee` · **Read-only listing; kill is destructive · gated** · `ProcessKill` · 🔴. **What it does:** Detects misbehaving processes and offers to kill the killable ones. **How it works:** Detects **runaway** processes (sustained high CPU with low I/O) — the genuinely useful case — and offers a confirmed `SIGTERM` → `SIGKILL`. **Correction:** a true **zombie** (state `Z`) is *already dead*, a process-table entry awaiting reaping by its parent, and **cannot be killed with `kill -9`** — so for zombies the inspector reports the **parent PID** to act on rather than uselessly signaling the zombie. All kills are listed, confirmed through the gate, and validated with **privilege-delegated** identity checks (PID \+ birth time \+ executable path, validated and signaled atomically under elevation; see §4.7), which defeats PID reuse.

### **12.9 Thermal & Fan Telemetry**

**Command:** part of `suns get-coffee` / `suns monitor` · **Read-only · may need root.** **What it does:** Live CPU/GPU temperatures and fan speeds. **How it works:** On Apple Silicon, reads a supervised, **time-bounded** `powermetrics` stream or IOKit (`IOHIDEventSystemClient`); on Intel, the legacy SMC. Uses the **embedded board-ID → sensor-key manifest** because sensor keys differ across M-series generations; shows "N/A" on fanless or unknown hardware; carries a staleness badge if the source wedges; and is bounded as a Cgo path per §4.4. `powermetrics` needs root via the §6 chokepoint.

### **12.10 Memory Pressure Visualizer**

**Command:** part of `suns get-coffee` / `suns monitor` · **Read-only.** **What it does:** Shows real memory pressure, not merely free-versus-used. **How it works:** Pulls Wired / App / Compressed / Swap breakdowns from `vm_stat`, `sysctl vm.swapusage`, and gopsutil, and surfaces the macOS **pressure state** (green/yellow/red) — the holistic signal that actually predicts slowdowns — with a swap-in/out rate sparkline.

### **12.11 DNS Cache Incinerator**

**Command:** part of `suns clean` (or `suns dns-flush`) · **Destructive(action) · gated** · `CacheReset` · 🔴 (no undo) · root. **What it does:** Flushes the system DNS cache. **How it works:** Runs `dscacheutil -flushcache; killall -HUP mDNSResponder` via the §6 chokepoint. It is modeled as a `CacheReset` operation: irreversible (there is nothing to "undo") but harmless — the only cost is that the next handful of lookups are marginally slower as the cache repopulates, and any in-flight resolutions reset. Because it is a *reset* rather than a deletion, the Jarjar deletion axis is **inert** for it; it carries the 🔐 root badge. It is useful after editing `/etc/hosts`, changing DNS records, switching VPNs, or debugging stale routing.

### **12.12 Open Port Auditor**

**Command:** `suns net ports` · **Read-only.** **What it does:** Shows what is listening and whether it is reachable from outside the machine. **How it works:** Maps sockets in the `LISTEN` state to their PID and application, and flags `0.0.0.0`/`::`\-bound (externally reachable) ports versus loopback-only ports, with a security-severity color. This is excellent for catching an accidentally-exposed development database or server.

### **12.13 SIP / Security Checker**

**Command:** `suns audit` · **Read-only.** **What it does:** A dashboard of the Mac's core security settings. **How it works:** Parses `csrutil status` (System Integrity Protection), `spctl --status` (Gatekeeper), and `fdesetup status` (FileVault) into a single posture view, adding Gatekeeper assessment and, where readable, secure-boot and XProtect version. These are native binaries with stable output, so the readings are accurate.

### **12.14 Sudo / Auth Log Analyzer**

**Command:** `suns audit logs` · **Read-only · root.** **What it does:** Surfaces failed authentication and privilege-escalation attempts. **How it works:** **Correction:** `/var/log/system.log` is largely deprecated in favor of the unified logging system, so this queries `log show --predicate 'process == "sudo"' --last 1d` (and the `Authorization`/`opendirectoryd` predicates) for failed escalations and auth failures, rendering a timeline with rapid-failure highlighting. The query needs root via the §6 chokepoint.

### **12.15 Docker Environment Nuke**

**Command:** part of `suns clean` (or `suns docker-prune`) · **Destructive · gated** · `ContainerPrune` · 🔴. **What it does:** Reclaims space from unused Docker images, containers, and volumes. **How it works:** **Correction:** Docker does not run natively on macOS — it runs inside a Linux VM (Docker Desktop, Colima, or OrbStack) — so the Go SDK's default Unix socket path often does not exist. Suns therefore **probes for `~/.docker/run/docker.sock`, `~/.colima/default/docker.sock`, and OrbStack's endpoint** before initializing the SDK client (falling back to the CLI). It previews reclaimable space, then runs `system prune -a --volumes` **only after the gate confirms**. It detects whether the daemon is running first and gives a clear no-op message if Docker is absent.

#### **12.15-uninstaller. Precision Uninstaller (payload-aware, shared-dependency-safe)**

**Command:** `suns nuke <app>` · **Destructive · gated** · `FileDelete` \+ `ServiceUnload` \+ `ReceiptForget`. **What it does:** Removes an application and its traces without orphaning files or breaking shared dependencies. **How it works:** Finds the application (in `/Applications`, `~/Applications`, or subfolders), reads its `Info.plist` (**binary-safe**) for the `CFBundleIdentifier`, and traces support files across `~/Library` by matching the bundle identifier (a heuristic that is imperfect — apps do not always name their support files by bundle ID — and is therefore best-effort). For applications installed via a `.pkg`, the teardown is done **in a specific, corrected order**: (1) `pkgutil --pkgs` to find the receipts; (2) **`pkgutil --files <id>`** to enumerate the payload (because `pkgutil --forget` removes only the *receipt* and would otherwise leave the payload — daemons in `/Library/LaunchDaemons`, binaries in `/usr/local/bin` — permanently orphaned); (3) **for each harvested absolute path, `pkgutil --file-info <path>`**, and if the file is claimed by **more than one installed package ID**, **exclude it** and mark it "Retained (shared dependency)" in the history (this prevents bricking unrelated applications by deleting a shared `dylib` or Audio Unit); (4) generate `FileDelete` and `ServiceUnload` operations for the remaining sole-owned payload; (5) only then run `pkgutil --forget`. It also surfaces `~/Library` remnants and related launch agents. **Explicitly bounded:** it does **not** comprehensively chase Mac App Store containers, app-group containers, shared frameworks used by other apps, login items, helper tools, or all LaunchServices registrations — it reports what it will and will not touch, and the UI never claims a "complete uninstall."

### **12.16 Dev-Cache Immolator**

**Command:** part of `suns clean` · **Destructive · gated** · `FileDelete` · 🟡 (recoverable). **What it does:** Clears developer-tool caches that safely rebuild. **How it works:** Targets a named, safe set of caches, each carrying a **rebuild-cost tag**: npm/pnpm/yarn caches, the Go module cache (`~/go/pkg/mod/cache`), pip, **Xcode `DerivedData`**, CocoaPods, Carthage, the SwiftPM cache, Gradle and Android build caches, and Homebrew downloads. The Apple-developer caches (DerivedData in particular) are typically the largest single wins (often 20–60 GB). **`iOS DeviceSupport` is split out** into its own opt-in category with a TUI warning: deleting it means the next time a physical iOS device is connected, Xcode hangs for 10–20 minutes re-extracting symbol files — a significant workflow disruption. More broadly, every "expensive-to-rebuild" cache is presented in **user-facing language about practical disruption**, because a 20-minute rebuild *feels* like data loss even when it is technically recoverable (§13.5).

### **12.17 Git Repository Garbage Collector**

**Command:** `suns maintain` · **Destructive · gated** · `RepoMaintenance` · 🟡 · **de-fanged.** **What it does:** Runs safe garbage collection across git repositories. **How it works:** **Correction:** the original `git gc --aggressive --prune=now` across *all* repositories can permanently drop recoverable work — dropped stashes, reset commits, abandoned branches — because `--prune=now` removes the grace period and aggressive collection rewrites and prunes the reflog. The default is therefore a **plain `git gc`** with the normal two-week prune window; `--aggressive` and immediate prune are explicit opt-ins with a per-repo warning. It discovers repositories, lists each with estimated savings **before** acting, **skips repositories with uncommitted changes or in-progress merge/rebase operations**, and gates the whole batch.

### **12.18 Broken Symlink Auditor**

**Command:** `suns scan` (audit, read-only) / `suns clean` (destroy, gated · `FileDelete` · 🟢). **What it does:** Finds (and optionally removes) symlinks that point to nothing. **How it works:** Resolves symlinks across the targeted roots and flags the dangling ones. Audit mode lists them read-only; destroy mode routes the removals through the gate using the fd-anchored, no-follow deleter (§4.6).

### **12.19 Empty Directory Purger**

**Command:** part of `suns clean` · **Destructive · gated** · `FileDelete` · 🟢. **What it does:** Removes empty directories, cascading upward. **How it works:** A **post-order** (bottom-up) traversal, so that directories emptied by removing their children are themselves caught in the same pass. It **always confirms the target root scope first** ("Purge empty dirs under `<path>`? \[y/N\]") — independent of `confirm_mode` — and then operates only within the approved scope. An edge case it must decide consistently is whether a directory containing only a `.DS_Store` counts as empty.

### **12.20 Scheduled Burn Daemon**

**Command:** `suns schedule` · **Destructive(authoring) · constrained.** **What it does:** Sets up unattended periodic cleanups, locked to the safest settings. **How it works:** **Correction:** Apple deprecated user cron in favor of `launchd`, so this authors a **`launchd` user LaunchAgent** (an `SMAppService`\-installed agent if a future app-bundle distribution exists) running `suns clean --scheduled`. Unattended deletion is the most dangerous mode, so scheduled runs are the most constrained: **locked to the curated safe-target allowlist**, **`deletion_mode` forced to `trash`** (never obliterate unattended), interactive flags and config ignored, full operation-history logging, and a note that the agent's context must have Full Disk Access or it will silently skip protected paths. **Failure semantics:** a run that hits repeated permission denials, an unavailable target, or stale exclusions writes a `scheduled_run` history record with status `partial` or `failed`, and the failure is **surfaced prominently in both the CLI and the TUI** — never degrading silently. **No-GUI-session behavior:** when run with no active Aqua (login) session — where notifications cannot be shown and Trash semantics may differ in a headless user context — the job still forces trash (using the pure-Go `~/.Trash` fallback if the Cocoa API is unavailable headless, **never** a permanent delete), surfaces failures to the history log plus a notification queued for the next login, and **skips** any target whose trashability cannot be guaranteed in that context rather than escalating to permanent deletion.

---

## **13\. Cross-Cutting Concerns**

### **13.1 External-command parsing contracts**

Suns scrapes a substantial set of system tools — `nettop`, `lsof`, `log show`, `powermetrics`, `vm_stat`, `sysctl`, `pmset`, `iostat`, `csrutil`, `spctl`, `fdesetup`, `mdutil`, plus Docker and Git. Parser breakage and OS drift are major operational risks for a tool that depends on this much external output. For **each** scraped tool, `pkg/syscmd` defines a **parsing contract**: the expected output format, a parser with **version detection** (reading the tool or OS version where available), and **graceful degradation** (a parser that no longer matches surfaces "telemetry unavailable on this OS version" rather than emitting garbage). Where a tool offers **structured output** (plist or JSON: `powermetrics -f plist`, `log show --style json`, `system_profiler -json`), that is preferred over text scraping. Every parser is unit-tested against captured fixture output, per macOS version (§15).

### **13.2 Error handling**

Per-path I/O and permission errors are **collected, not fatal**, and surfaced as a "skipped" summary with reasons: no-permission, in-use, vanished, changed-identity, unreachable-volume, or retained-shared-dependency. TCC denials get a dedicated, actionable message (with the link to the Full Disk Access settings pane), never a stack trace. The `syscmd` wrapper surfaces non-zero command exits cleanly with bounded captured output. Operation execution failures are recorded with their status in the operation history.

### **13.3 Logging, history & privacy**

* **Application log:** `~/Library/Logs/Suns/suns.log`, written via `log/slog`, rotated, controlled by `--verbose` / `--quiet`. This is the macOS-native location where support workflows expect to find logs (chosen over an XDG-style `~/.local/state` path for that reason).  
* **Operation history (canonical):** `~/Library/Application Support/Suns/history.jsonl`, mode `0600`, **full fidelity** (real absolute paths and identities) because restore and debugging depend on it. **Never redacted in place.** It is an operational record, **not** a tamper-evident forensic trail, and is described as such (no cryptographic chaining or signing in v1).  
* **Exported support bundle (redacted):** a separate, explicit `suns history export --redact` produces a **redacted copy** (home-relative path hashing or truncation, process-name elision) for sharing, leaving the canonical record intact. This split is deliberate: redaction protects privacy on export without weakening the recovery and debugging fidelity that depends on the canonical record. (Paths, repo roots, and process names can otherwise reveal usernames and project context.)  
* **Retention:** the canonical history rotates and expires on a size cap (old logs gzip into dated archives).  
* **Neutral machine keys, fun UI names:** configuration files and log records use neutral, scriptable keys (`confirm_mode`, `deletion_mode`, `file_delete`, `process_kill`, …); the memorable names (Deathstar, Jarjar, "Burn It All") live **only in the UI**. This is the best of both worlds — a memorable surface and a clear, scriptable machine layer.

### **13.4 Scriptability**

Destructive commands support `--dry-run` (prints the **frozen plan** as text or JSON, exits 0, takes no action) and `--yes` (confirmation bypass). Read-only commands support `--json`. **Dry-run parity is defined precisely:** the dry-run plan equals the **planned** set; a real run may additionally **skip** items that fail execution-time revalidation (vanished, replaced, identity-changed, or unreachable) and reports them. Parity is therefore between the dry-run plan and the *planned* set — never a promise that every planned item is acted upon (which would contradict the safety model's revalidation skips).

### **13.5 Cache classification**

"Reclaimable" is not the same as "cheap to rebuild." Each safe-cache target carries a **rebuild-cost tag** — cheap, moderate, or expensive-or-disruptive. Expensive or disruptive targets (iOS DeviceSupport, large simulators) are opt-in with warnings phrased in user-facing terms (a 10–20 minute Xcode symbol re-extraction *feels* like data loss even when technically recoverable), and are never part of the default sweep.

### **13.6 Honest performance framing**

"Never blocked," "\~10 Hz," and "approximately zero per-tick allocation at steady state" are **goals and steady-state intentions, not guarantees.** A TUI that also drives heavy system binaries can stall. The double-buffered snapshot model, the supervised-and-deadline-guarded streams, and the bounded-and-degrading native calls make stalls rare and bounded — but the document states these as design intentions rather than promises.

### **13.7 Safety-floor invariance**

The never-touch deny floor (§5.3) is enforced inside `pkg/safety` at **plan time, execution time, and every directory descent**, and **cannot be bypassed** by any flag, mode, config value, or scheduled job.

---

## **14\. Implementation Roadmap & Product Scope**

The build is CLI-first and terminal-native (matching the intended operator's workflow). Each phase is independently shippable and testable, and the **dangerous primitives are built and proven first**, before any feature leans on them.

### **Phase 0 — The dangerous core, proven first (the v1 spine)**

Build and prove, before anything destructive ships: `pkg/operation` (value-type concrete kinds) and `pkg/plan` (value-`Seal`); `pkg/safety` with the gate, the deny-floor, the `fsdelete` fd-anchored recursive deleter **plus the firmlink layer**, and the tiered identity-aware `revalidate`; `pkg/trash` with **bounded Cgo, the killable volume probe, the semaphore, the circuit breaker, and the pure-Go fallback**, validated with real hung-volume tests; `pkg/history` (`0600` canonical plus export-redaction); the hardened `pkg/syscmd`; `pkg/privilege` (the per-workflow `sudo` chokepoint with TTY-ticket and failure-state handling, **no persistent runner**); and `suns doctor`. *Nothing destructive ships until the frozen value-plan, the fd-anchored delete with firmlink handling, the tiered identity validation, the bounded-and-degrading trash, and the elevation chokepoint are all proven.*

### **Phase 1 — v1 ships: `clean` (safe-cache allowlist) \+ `get-coffee`**

Concurrent `WalkDir` discovery into a **double-buffered** `ScanSnapshot`; the safe-cache manifest with rebuild-cost tags; `suns clean` (caches, logs, dev-caches, empty dirs) end-to-end as a sealed plan of `FileDelete` operations; `suns restore` with tiered identity and fail-loud collision rules; the supervised telemetry poller with the **deadline-guarded powermetrics tokenizer**, bounded IOKit, and staleness badges; the nine read-only dashboard widgets; the gated, privilege-delegated process kill. The TUI shell, theme and mode recoloring, the Apple\_Terminal sparkline fallback, and the frozen-plan confirmation modal. **This is a complete, trustworthy, low-surface product** — a beautiful live dashboard plus a safe cache cleaner with undo, touching none of the high-correctness-surface features.

### **Phase 2 — Uninstaller & dedup**

`suns nuke` (binary plist parsing, bundle-ID tracing, the **`pkgutil --file-info` shared-dependency guard**, the harvest-then-forget ordering, and the explicitly bounded scope); the APFS-conservative `suns dedup` with the xattr-normalization policy.

### **Phase 3 — Net & audit suites**

`suns net` (socket mapper, open-port auditor, bandwidth detector, LAN scan); `suns audit` (SIP/Gatekeeper/FileVault posture, auth-log analysis). The parsing-contract layer matures here; `nettop` per-PID remains experimental.

### **Phase 4 — Maintenance, scheduling, release pipeline**

`suns maintain` (de-fanged git gc), Docker prune (socket-probed), the empty-directory and broken-symlink destroyers, DNS flush; `suns schedule` (launchd, with no-GUI-session semantics and prominent failure surfacing); the quarantined `suns lang-strip` (last). **Release engineering as a first-class task:** the universal binary build, the Cgo build, code signing, notarization, `SMAppService` registration if pursued, and the Homebrew tap.

---

## **15\. Testing & Safety Validation**

Because Suns deletes data, testing is a safety feature, not a chore.

* **Sandbox guard.** All destructive tests run against a generated temporary directory tree (fake caches, duplicates, broken symlinks, nested empty directories). A test guard asserts that **no path outside the sandbox is ever passed to a delete call.**  
* **Bounded-and-degrading trash (the headline test).** Simulate a hung volume (a FUSE mount that blocks) and assert: the OS-thread count stays bounded (no runaway toward the runtime's thread limit), the circuit breaker trips, and **`FileDelete` on a *healthy* volume still succeeds via the fallback** (no global bricking). Assert the cross-volume fallback uses copy-then-unlink and **never a permanent delete**. Assert the volume probe runs in a killable helper (or, where it cannot, that the accepted-loss path remains bounded by the breaker and fallback).  
* **Double-buffering.** A synthetic million-file scan asserts **approximately zero per-tick allocation at steady state** (via an allocation counter), no scanner stall under heavy UI reads, and no stop-the-world spikes beyond budget — validating the `atomic.Pointer` double-buffer model.  
* **fd-anchored delete \+ firmlink.** Inject a directory-to-symlink swap mid-descent and assert deletion never escapes the subtree; assert that a firmlink crossing (`/var`, `/Users`) is **permitted** (a legitimate path is not falsely rejected) while a user-space symlink redirection **is** rejected.  
* **Value-sealed plan.** Mutate scanner and session state after `Seal()` and assert the plan is byte-identical.  
* **Identity validation.** PID reuse (recycle a PID with a different birth time → the kill is refused, **under elevation**); a swapped Trash entry (different identity → restore refused); a relabeled service (→ unload refused); **tiered behavior** (a large file is validated without a full hash; a small high-risk file is validated by hash).  
* **Shared-dependency uninstall.** A payload file claimed by two packages → excluded and marked "Retained (shared dependency)"; a sole-owned file → deleted.  
* **Gate matrix.** All four file-deletion mode combinations are verified — confirm-off blocks, trash mode never calls `os.RemoveAll`, and non-file operations correctly ignore the deletion axis while showing 🔴.  
* **Floor tests.** The deny-list rejects `/System`, `~`, and the rest under **every** mode/flag/config permutation, at plan, exec, **and** descent.  
* **Restore.** Round-trip plus the collision cases (occupied path, missing parent, permission-denied), and an assertion that inert relocation never happens silently (the fail-loud path is verified).  
* **Telemetry supervision.** Wedge/EOF a stream and assert restart with backoff, the `stale`/`unavailable` state, the staleness badge, and parser resync after a malformed block; the **powermetrics tokenizer** fed concatenated documents asserts each decodes, and a **mid-document stall** asserts the read-deadline flush-plus-stale-plus-restart; a **hung IOKit read** asserts it is bounded and marked stale.  
* **Privilege.** Allowlist enforcement (a non-allowlisted command is a test failure); argument-injection attempts rejected; `LC_ALL=C` set; no shell invoked; the canceled-prompt, expired-ticket, and partial-batch failure paths covered; the **privileged ProcessKill validation** runs under elevation; and **no persistent root process exists** after a workflow completes.  
* **APFS dedup.** Hardlink, clone/shared-block (assert it is **not** wrongly excluded), bundle, and xattr-normalization fixtures.  
* **Plist fuzz.** Binary, XML, and malformed plists.  
* **Golden TUI tests.** `teatest` snapshots of the key views in both color modes and the Apple\_Terminal fallback.  
* **Dry-run parity.** The dry-run plan equals the planned set; revalidation skips are reported (and are not treated as a parity violation).  
* **Concurrency.** `goleak` (no orphaned workers) and the race detector (`-race`) on all tests.  
* **CI.** `go vet`, `staticcheck`, `golangci-lint`, and `-race`.

---

## **16\. Appendices**

### **16.1 Consolidated review-item disposition (v1 → v6)**

Every substantive issue raised across all review rounds, and where it is resolved.

| Issue (origin) | Disposition | § |
| ----- | ----- | ----- |
| `rjeczalik/notify` cited for Trash (it is an FS-event library) | Fixed — Cgo `NSFileManager trashItemAtURL:` | §2.4, §4.4 |
| `godirwalk` archived; "faster than Walk" stale | Fixed — stdlib `filepath.WalkDir` | §2.3 |
| Assumed all plists are XML | Fixed — `howett.net/plist` (binary-safe) \+ `plutil` | §2.4 |
| "kill \-9 a zombie" (impossible) | Fixed — detect runaways; report zombie parent PID | §12.8 |
| `git gc --aggressive --prune=now` everywhere (data loss) | Fixed — default safe gc; opt-in aggressive; list-first; skip dirty | §12.17 |
| Cron scheduling (deprecated) | Fixed — `launchd` via SMAppService | §12.20 |
| SMC thermals (Intel-only) | Fixed — IOKit/`powermetrics` on Apple Silicon; embedded sensor map | §5.2, §12.9 |
| `/var/log/system.log` (deprecated) | Fixed — unified log `log show --predicate` | §12.14 |
| No mention of TCC/Full Disk Access | Fixed — detect, guide, degrade | §5.1 |
| No privilege model | Fixed — centralized per-workflow sudo chokepoint | §6 |
| Blanket `~/Library/Caches` deletion unsafe | Fixed — curated allowlist \+ exclusions | §5.3 |
| UI treated as afterthought | Fixed — full design system \+ dashboard | §10, §11 |
| Safety model path-centric; product is not | Fixed — generalized Operation model | §4.1, §4.2 |
| Reversibility claim overstated | Fixed — per-kind reversibility class \+ typed history | §4.2, §4.10 |
| TOCTOU gap | Fixed — execution-time revalidation, `O_NOFOLLOW`, fd-anchored | §4.5, §4.6, §4.7 |
| osascript privilege footgun | Fixed — sudo chokepoint via `tea.Exec`; osascript not primary | §6 |
| Trash fallback vs restore conflict | Fixed — restore uses our own log, not OS Put-Back | §4.4, §4.8 |
| Cgo can't be ctx-preempted | Fixed — isolated, bounded, with fallback | §4.4 |
| Event-bus saturation | Fixed — double-buffered pull model | §3.3, §3.2 |
| powermetrics/nettop per-tick spawn | Fixed — long-lived streams \+ sysctl | §7 |
| pkgutil receipt ghosts (orphaned payload) | Fixed — `--files` harvest then `--forget` | §12.15-uninstaller |
| Viper not thread-safe | Fixed — RWMutex SessionState | §4.9 |
| APFS dedup semantics (clone over-claim) | Fixed — conservative; don't exclude; flag uncertainty; xattr policy | §12.1 |
| Keeper heuristic naive | Fixed — location-aware heuristic | §12.1 |
| Launch-agent false positives | Fixed — wrapper/script/relative resolution; bounded | §12.3 |
| Restore collision behavior | Fixed — explicit rules; fail-loud | §4.8 |
| Docker socket on macOS | Fixed — probe Desktop/Colima/OrbStack | §12.15 |
| iOS DeviceSupport disruptive | Fixed — split out, opt-in, warned | §12.16 |
| External-command fragility / syscmd underspec | Fixed — hardened syscmd \+ parsing contracts | §6.3, §13.1 |
| Scheduled-run auditability | Fixed — failure semantics \+ summary record | §12.20 |
| Cgo trash detached goroutine leaks OS threads | Fixed — probe \+ semaphore \+ breaker | §4.4 |
| XPC impractical from Go / wrong for Homebrew | Fixed — sudo primary; XPC demoted optional | §6 |
| RWMutex held across heavy read stalls scanner | Fixed — `atomic.Pointer` immutable snapshots | §3.2 |
| Execution reads live aggregate; plan drift | Fixed — frozen `Plan.Seal()` | §4.5 |
| `os.RemoveAll` not TOCTOU-safe for dirs | Fixed — fd-anchored `openat`/`unlinkat` | §4.6 |
| Validation only kind-aware; PID reuse | Fixed — identity-aware `ValidateAtExec` | §4.7 |
| `pkgutil --forget` orphans payload | Fixed — `--files` harvest → ops → forget | §12.15-uninstaller |
| powermetrics plist concatenation choke | Fixed — stream tokenizer | §7.3 |
| Telemetry supervision absent | Fixed — supervised streams, staleness badges | §7.4 |
| "Audit" without integrity (misleading) | Fixed — renamed operation history; 0600; not forensic | §4.10, §13.3 |
| Log privacy/redaction | Fixed — canonical full vs export-redact split | §13.3 |
| Inert restore relocation \= fake success | Fixed — fail loud; relocation only by explicit choice | §4.8 |
| Dry-run exact-parity contradiction | Fixed — plan↔planned; skips reported | §13.4 |
| TTY-ticket silent-inherit for bg streams | Fixed — elevated-session inheritance | §6.1 |
| tea.Exec sudo failure states | Fixed — canceled/expired/partial defined | §6.1 |
| Privilege boundary under-specified | Fixed — discovery unprivileged; chokepoint \= narrow root actions | §6.4 |
| FDA inference too reactive | Fixed (balanced) — bounded startup probe \+ per-path EPERM | §5.1 |
| Code-signing doesn't grant FDA | Fixed — honest framing | §5.1 |
| Performance claims too absolute | Fixed — framed as goals | §13.6 |
| Trash semaphore bricks subsystem | Fixed — graceful pure-Go fallback | §4.4 |
| Volume probe can block in-kernel | Fixed — killable helper / accepted bounded loss | §4.4 |
| Boundedness claim broader than mechanism (IOKit) | Fixed — IOKit containment | §4.4, §7.4 |
| GC thrash from snapshot churn | Fixed — double-buffering; arena rejected | §3.2 |
| `pkgutil --files` deletes shared deps | Fixed — `--file-info` shared-dependency guard | §12.15-uninstaller |
| Process-validation privilege inversion | Fixed — delegated to privilege chokepoint | §4.7, §6.4 |
| Full content hash on restore too slow | Fixed — tiered identity | §4.7, §4.8 |
| Tokenizer mid-document stall | Fixed — read-deadline → flush+stale+restart | §7.3 |
| Firmlink traversal false-reject | Fixed — firmlink-aware identity layer | §4.6 |
| `Plan.Seal()` only aspirational | Fixed — value-sealed, pointer-free, test-enforced | §4.5 |
| Persistent elevated runner under-specified | Fixed — dropped; per-workflow tickets only | §6.1 |
| Trash vs fd-delete dir semantics unclear | Fixed — trash \= atomic-at-root; fd-walker obliterate-only | §4.6 |
| Scheduled run, no GUI session | Fixed — defined headless/trash/notification/skip semantics | §12.20 |
| nettop per-PID fragility | Fixed — demoted experimental; interface totals core | §7.5 |
| v1 scope too wide | Fixed — explicit scope ladder; minimal v1 | §1.3, §14 |
| Uninstaller/launch-agent over-claimed | Fixed — best-effort, explicitly bounded, UI-honest | §12.3, §12.15, §10.7 |
| Branding pulls against safety | Held (deliberate) — fun names UI-only; neutral machine keys | §13.3 |
| Remote IOKit manifest / sandbox-exec FDA / `.DS_Store` hack / arena allocator / XPC-primary / persistent runner | Rejected (with reasons) | §2.7 |

### **16.2 Glossary**

* **Operation** — a typed, auditable, potentially-destructive action (the unit of the safety model): FileDelete, ProcessKill, CacheReset, ServiceUnload, RepoMaintenance, ContainerPrune, ReceiptForget, DNSFlush.  
* **Plan (value-sealed)** — a deep-copied, pointer-free, immutable set of operations that the gate confirms and the executor consumes as a unit; immutability is enforced by test.  
* **Reversibility** — the Reversible / Recoverable / Irreversible classification carried by every operation and surfaced as 🟢 / 🟡 / 🔴.  
* **Identity (validation)** — execution-time proof that a target is still what was planned: device \+ inode for files, PID \+ birth time \+ executable path for processes, domain \+ label \+ plist for services, content hash for small high-risk files.  
* **Tiered identity** — size \+ mtime \+ inode for large files, full content hash for small high-risk files, so verification never blocks on huge objects.  
* **fd-anchored delete** — recursive deletion via directory file descriptors (`openat` / `unlinkat`), immune to mid-descent component swaps.  
* **Graceful degradation (trash)** — falling back to a pure-Go `~/.Trash` move when the bounded native Trash is unavailable, so the subsystem never bricks and never permanent-deletes as a "fallback."  
* **Double-buffering** — two reused backing arrays swapped via `atomic.Pointer` for approximately zero-allocation UI snapshots.  
* **Firmlink** — a macOS System/Data volume bind (`/var`, `/Users`) that crosses device numbers but is OS-synthesized, not a user symlink; permitted by the identity layer.  
* **Shared-dependency retention** — excluding files claimed by more than one installed package from an uninstall, to avoid breaking unrelated apps.  
* **Confirm mode (Deathstar)** — the `confirm_mode` axis: preview-and-gate versus immediate execution.  
* **Deletion mode (Jarjar)** — the `deletion_mode` axis (FileDelete only): move-to-Trash versus permanent obliterate.  
* **Operation history** — the `0600` JSONL operational record (full-fidelity canonical, export-only redaction); not a tamper-evident forensic trail.  
* **TCC / Full Disk Access** — macOS privacy permissions controlling access to protected directories.  
* **APFS block sharing** — copy-on-write reference counting; it does not identify clone lineage, so dedup treats it conservatively.

---

*Suns v6 — the consolidated master document. Safe by default, dangerous on demand, bounded in blast radius, degrading rather than bricking, and honest about every guarantee.*

*Burn It All — but only what you meant to, only what you can answer for, never more than the runtime can survive, and never the dylib three other apps were using.*

