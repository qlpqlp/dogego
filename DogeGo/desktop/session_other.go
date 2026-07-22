//go:build !windows && !linux && !darwin

package desktop

func interactiveSession() bool {
	return false
}
