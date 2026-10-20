//go:build !windows && !darwin && !linux

package capture

import "errors"

func activeWindow() (*Result, error) {
	return nil, errors.New("window capture not supported on this OS")
}
