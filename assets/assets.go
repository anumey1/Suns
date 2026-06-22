// Package assets embeds the static data the binary ships with, so Suns is fully
// self-contained (§8): the ASCII sun logo, the MAC-vendor OUI lookup table for
// the LAN scanner, the safe-cache allowlist manifest, and the board-ID →
// sensor-key map. None of these are fetched remotely — the sensor manifest in
// particular is embedded deliberately to avoid adding a network/update-trust
// surface to a security-sensitive tool (§2.7, §5.2).
package assets

import _ "embed"

//go:embed logo.txt
var Logo string

//go:embed safe_cache.json
var SafeCacheManifest []byte

//go:embed sensors.json
var SensorManifest []byte

//go:embed oui.csv
var OUIDatabase []byte
