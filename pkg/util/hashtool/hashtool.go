package hashtool

import "hash/fnv"

// Hash32 hashes key to a uint32.
func Hash32(key string) uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(key))
	return hash.Sum32()
}
