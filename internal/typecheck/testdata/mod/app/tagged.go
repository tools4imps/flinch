//go:build extra

package app

import "example.com/mod/lib"

func Tagged() int { return lib.Extra() }
