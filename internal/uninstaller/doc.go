// Package uninstaller is the precision uninstaller backing `suns nuke <app>`
// (§12.15-uninstaller). Destructive · gated · FileDelete + ServiceUnload +
// ReceiptForget.
//
// It traces an app by binary-safe Info.plist CFBundleIdentifier, then for
// .pkg-installed apps tears down in a corrected order: pkgutil --pkgs → pkgutil
// --files <id> (harvest the payload, because --forget removes only the receipt
// and would orphan it) → pkgutil --file-info <path> for each harvested path,
// EXCLUDING any file claimed by more than one installed package ID and marking
// it "Retained (shared dependency)" so unrelated apps are not bricked → generate
// FileDelete/ServiceUnload ops for the sole-owned payload → only then pkgutil
// --forget. Explicitly bounded: it reports what it will and will not touch and
// never claims a "complete uninstall" (§10.7).
package uninstaller
