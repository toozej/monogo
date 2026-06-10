// Package starter provides the core functionality for the golang-starter application.
//
// This package contains the main business logic that demonstrates a simple greeting
// functionality. It serves as a template for implementing core application features
// in a golang-starter project.
//
// The package is designed to be called from the command-line interface and can
// be easily extended with additional functionality as needed for specific use cases.
//
// Example usage:
//
//	import "github.com/toozej/golang-starter/internal/starter"
//
//	func main() {
//		starter.Run("Alice")
//		// Output: Hello from Alice
//	}
package starter

import (
	"fmt"
)

// Run executes the main functionality of the starter package by printing
// a personalized greeting message to standard output.
//
// This function demonstrates basic I/O operations and serves as a template
// for implementing core application logic. The greeting format is:
// "Hello from <username>"
//
// Parameters:
//   - username: The name to include in the greeting message. Can be any string,
//     including empty strings or strings containing whitespace.
//
// Example:
//
//	starter.Run("Alice")
//	// Output: Hello from Alice
//
//	starter.Run("")
//	// Output: Hello from
//
//	starter.Run("John Doe")
//	// Output: Hello from John Doe
func Run(username string) {
	fmt.Println("Hello from", username)
}
