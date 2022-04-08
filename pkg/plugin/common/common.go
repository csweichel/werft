package common

// Type denotes the plugin type
type Type string

const (
	// TypeIntegration means the plugin can act as integration plugin
	TypeIntegration Type = "integration"

	// TypeRepository means the plugin can add support for remote repositories (e.g. GitHub)
	TypeRepository Type = "repository"

	// TypeAuthentication means the plugin can add support for authenticating requests (e.g. against GitHub)
	TypeAuthentication Type = "auth"
)
