package cmd

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func readPassword(pw string) (string, error) {
	if pw != "" {
		return pw, nil
	}

	fmt.Print("Enter password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	return string(b), err
}
