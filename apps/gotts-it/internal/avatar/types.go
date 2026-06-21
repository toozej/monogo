// Package avatar provides terminal image rendering for the gotts-it application.
//
// This package renders the project's mascot avatar in the terminal using
// the go-termimg library. It supports rendering from file paths or URLs,
// and provides graceful error messages when the terminal does not support
// image rendering.

package avatar

// DefaultAvatarURL is the URL to the project's GitHub avatar/mascot image.
const DefaultAvatarURL = "https://github.com/toozej.png"

// DefaultAvatarPath is a local fallback path for the avatar image.
const DefaultAvatarPath = "./img/avatar.png"
