// Package optimizer wraps reset-style action operations such as the DNS cache
// flush and mdutil (§12.11).
//
// The DNS flush runs `dscacheutil -flushcache; killall -HUP mDNSResponder` via
// the §6 privilege chokepoint, modeled as a CacheReset operation: irreversible
// (nothing to undo) but harmless, so the Jarjar deletion axis is inert for it;
// it carries the 🔐 root badge.
package optimizer
