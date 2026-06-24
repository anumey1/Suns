package audit

import "strings"

// Parsing contracts for the security-posture tools (§13.1). Each tool prints
// stable, human-readable status text that has been consistent across many macOS
// releases; we still parse defensively (case-fold, substring match) and return
// an "unknown" finding rather than guessing when the text is unexpected. All
// three are unprivileged status reads run under the hardened executor.

// parseSIP reads `csrutil status`. Expected:
//
//	System Integrity Protection status: enabled.
//	System Integrity Protection status: disabled.
//	System Integrity Protection status: unknown (Custom Configuration).
func parseSIP(out []byte) Finding {
	s := strings.ToLower(string(out))
	switch {
	case s == "":
		return Finding{Name: "SIP", State: "unknown", Severity: SevUnknown, Detail: "csrutil unavailable"}
	case strings.Contains(s, "custom configuration"):
		return Finding{Name: "SIP", State: "custom", Severity: SevWarn, Detail: "System Integrity Protection has a custom (partial) configuration"}
	case strings.Contains(s, "status: enabled"):
		return Finding{Name: "SIP", State: "enabled", Severity: SevOK, Detail: "System Integrity Protection is enabled"}
	case strings.Contains(s, "status: disabled"):
		return Finding{Name: "SIP", State: "disabled", Severity: SevRisk, Detail: "System Integrity Protection is DISABLED"}
	default:
		return Finding{Name: "SIP", State: "unknown", Severity: SevUnknown, Detail: "unrecognized csrutil output"}
	}
}

// parseGatekeeper reads `spctl --status`. Expected:
//
//	assessments enabled
//	assessments disabled
func parseGatekeeper(out []byte) Finding {
	s := strings.ToLower(string(out))
	switch {
	case s == "":
		return Finding{Name: "Gatekeeper", State: "unknown", Severity: SevUnknown, Detail: "spctl unavailable"}
	case strings.Contains(s, "assessments enabled"):
		return Finding{Name: "Gatekeeper", State: "enabled", Severity: SevOK, Detail: "Gatekeeper assessments are enabled"}
	case strings.Contains(s, "assessments disabled"):
		return Finding{Name: "Gatekeeper", State: "disabled", Severity: SevRisk, Detail: "Gatekeeper assessments are DISABLED"}
	default:
		return Finding{Name: "Gatekeeper", State: "unknown", Severity: SevUnknown, Detail: "unrecognized spctl output"}
	}
}

// parseFileVault reads `fdesetup status`. Expected:
//
//	FileVault is On.
//	FileVault is Off.
//	FileVault is On but a deferred enablement appears to be active.
func parseFileVault(out []byte) Finding {
	s := strings.ToLower(string(out))
	switch {
	case s == "":
		return Finding{Name: "FileVault", State: "unknown", Severity: SevUnknown, Detail: "fdesetup unavailable"}
	case strings.Contains(s, "is on") && strings.Contains(s, "deferred"):
		return Finding{Name: "FileVault", State: "deferred", Severity: SevWarn, Detail: "FileVault enablement is deferred (not yet encrypting)"}
	case strings.Contains(s, "is on"):
		return Finding{Name: "FileVault", State: "on", Severity: SevOK, Detail: "FileVault is on"}
	case strings.Contains(s, "in progress") || strings.Contains(s, "encrypting"):
		return Finding{Name: "FileVault", State: "encrypting", Severity: SevWarn, Detail: "FileVault encryption is in progress"}
	case strings.Contains(s, "is off"):
		return Finding{Name: "FileVault", State: "off", Severity: SevWarn, Detail: "FileVault is OFF — disk is not encrypted at rest"}
	default:
		return Finding{Name: "FileVault", State: "unknown", Severity: SevUnknown, Detail: "unrecognized fdesetup output"}
	}
}
