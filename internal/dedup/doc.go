// Package dedup is the APFS-conservative duplicate finder backing `suns dedup`
// (§12.1). Destructive · gated · FileDelete.
//
// Three passes: group by exact byte size → hash the first 4 KB as a cheap
// discriminator → full SHA-256 on the survivors; the eventual delete uses
// tiered identity (§4.7). APFS correctness: block sharing does NOT identify
// clone lineage, so shared-block files are NOT excluded (that would skip real
// duplicates); sharing is detected only to estimate reclaimable space
// conservatively and to flag that freed space may be lower than nominal.
// Hardlinks (same inode) are never offered as deletable. Bundles (.app, .rtfd,
// .photoslibrary) are atomic — interior files are never deduped. Comparison is
// over data forks, ignoring cosmetic xattrs (quarantine, Finder tags,
// where-from). Keeper heuristic prefers non-cache, non-trash user-document
// locations and never auto-selects across bundles.
package dedup
