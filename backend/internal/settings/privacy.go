package settings

import "time"

// PrivacySection is deliberately presentation-neutral so native clients can
// show the same policy without copying browser-specific markup.
type PrivacySection struct {
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
	Items      []string `json:"items,omitempty"`
}

type PrivacyPolicy struct {
	Version       string           `json:"version"`
	Title         string           `json:"title"`
	Summary       string           `json:"summary"`
	EffectiveDate string           `json:"effectiveDate"`
	Accepted      bool             `json:"accepted"`
	AcceptedAt    *time.Time       `json:"acceptedAt,omitempty"`
	Sections      []PrivacySection `json:"sections"`
}

// Policy returns the canonical policy content used by the first-run screen
// and Settings. PRIVACY.md mirrors this wording for people reading the repo.
func Policy(values Values) PrivacyPolicy {
	accepted := values.PrivacyAcceptedAt != nil && values.PrivacyPolicyVersion == CurrentPrivacyPolicyVersion
	policy := PrivacyPolicy{
		Version:       CurrentPrivacyPolicyVersion,
		Title:         "SyncSpace privacy policy",
		Summary:       "SyncSpace finds and transfers files between devices on your local network. This page explains what nearby devices can see and what SyncSpace keeps on this computer.",
		EffectiveDate: "21 July 2026",
		Accepted:      accepted,
		Sections: []PrivacySection{
			{Title: "Nearby device discovery", Paragraphs: []string{"When discovery is on, SyncSpace looks for other SyncSpace devices on the same local network and advertises enough information for them to recognise this device."}, Items: []string{"Nearby devices may see your SyncSpace device name, hostname, platform, app version, local network address, availability, and supported transfer features.", "Discovery only shows that a device is nearby. It does not make that device trusted."}},
			{Title: "File transfers", Paragraphs: []string{"Files move directly between the devices taking part. Core transfers do not upload a copy to a SyncSpace cloud service."}, Items: []string{"Only files and folders you choose are sent.", "Incoming transfers need your approval.", "Received files are never opened or run automatically.", "Paired transfers use an authenticated TLS 1.3 connection."}},
			{Title: "Information kept on this device", Paragraphs: []string{"SyncSpace stores the information it needs locally so trust, preferences, and transfer history survive a restart."}, Items: []string{"A randomly generated device ID and protected cryptographic identity.", "Settings, trusted-device relationships, privacy acceptance, and transfer history.", "File names, sizes, status, hashes, timestamps, and your chosen receive location. File contents are not stored in the application database."}},
			{Title: "Network and external services", Paragraphs: []string{"SyncSpace uses local IP addresses and ports to connect devices. It does not use a MAC address as your identity and does not inspect unrelated browser history, passwords, messages, contacts, or files you have not selected."}, Items: []string{"Core discovery and transfers stay on the local network.", "This version has no analytics, advertising, crash-report upload, account service, relay, or internet fallback."}},
			{Title: "Your choices", Paragraphs: []string{"You stay in control of when this device can be found and when it can receive files."}, Items: []string{"Rename this device, turn discovery off, or stop new incoming offers in Settings.", "Reject incoming transfers, cancel active transfers, remove trusted devices, change the receive folder, and clear transfer history.", "Declining this policy keeps discovery, pairing, and peer transfers disabled."}},
		},
	}
	if accepted {
		policy.AcceptedAt = values.PrivacyAcceptedAt
	}
	return policy
}
