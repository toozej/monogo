//go:build !linux

package runner

import "errors"

func lockState(string) (func(), error) { return nil, errors.New("runner requires Linux") }
