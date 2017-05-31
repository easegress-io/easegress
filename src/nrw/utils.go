package nrw

import "fmt"

func u2b(u uint64) []byte {
	return []byte(fmt.Sprintf("%d", u))
}
