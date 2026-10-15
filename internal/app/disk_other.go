//go:build !unix

package app

// diskSpace is not measured on this system; the admin page hides the figure.
func diskSpace(string) (free, total int64) { return 0, 0 }
