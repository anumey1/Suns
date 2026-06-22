// Package net backs `suns net` (§12.5, §12.6, §12.7, §12.12). Read-only.
//
// It maps which app owns which live connection (lsof -i -n -P via the hardened
// executor, with cached async reverse-DNS), audits listening ports (flagging
// 0.0.0.0/:: externally-reachable binds vs loopback), discovers LAN devices
// (IP/MAC/vendor via the embedded OUI table/hostname, warning before active
// scanning on networks the operator may not own), and detects bandwidth hogs
// (per-process deltas from a long-lived `nettop -P -l 0` stream — experimental;
// sysctl interface totals are the reliable core).
package net
