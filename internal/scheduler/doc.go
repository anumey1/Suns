// Package scheduler authors the scheduled burn daemon backing `suns schedule`
// (§12.20). Destructive (authoring) · constrained.
//
// Apple deprecated user cron, so this authors a launchd user LaunchAgent
// running `suns clean --scheduled`. Unattended deletion is the most dangerous
// mode, so scheduled runs are the most constrained: locked to the curated
// safe-target allowlist, deletion_mode FORCED to trash (never obliterate
// unattended), interactive flags and config ignored, full history logging.
// Failure semantics: repeated permission denials / unavailable targets / stale
// exclusions write a scheduled_run record with status partial|failed, surfaced
// prominently in both CLI and TUI — never silently. No-GUI-session: still forces
// trash (pure-Go ~/.Trash fallback if Cocoa is unavailable headless, never a
// permanent delete) and skips any target whose trashability cannot be
// guaranteed rather than escalating to permanent deletion.
package scheduler
