package util

import "github.com/spaolacci/murmur3"

// HashKey hashes a string to an int value using MurmurHash3 algorithm
func HashKey(key string) int {
	const seed uint32 = 0x1234ABCD
	hash := murmur3.Sum64WithSeed([]byte(key), seed)

	hash ^= hash >> 32
	hash *= 0xc6a4a7935bd1e995
	hash ^= hash >> 47

	if hash < 0 {
		return int(-hash)
	}
	return int(hash)
}

// PatternMatch matches a string with a wildcard pattern.
// It supports following cases:
// - h?llo matches hello, hallo and hxllo
// - h*llo matches hllo and heeeello
// - h[ae]llo matches hello and hallo, but not hillo
// - h[^e]llo matches hallo, hbllo, ... but not hello
// - h[a-b]llo matches hallo and hbllo
// - Use \ to escape special characters if you want to match them verbatim./
func PatternMatch(pattern, src string) bool {
	patLen, srcLen := len(pattern), len(src)

	if patLen == 0 {
		return srcLen == 0
	}

	if srcLen == 0 {
		for i := 0; i < patLen; i++ {
			if pattern[i] != '*' {
				return false
			}
		}
		return true
	}

	patPos, srcPos := 0, 0
	for patPos < patLen {
		switch pattern[patPos] {
		case '*':
			for patPos < patLen && pattern[patPos] == '*' {
				patPos++
			}
			if patPos == patLen {
				return true
			}
			for srcPos < srcLen {
				for srcPos < srcLen && src[srcPos] != pattern[patPos] {
					srcPos++
				}
				if PatternMatch(pattern[patPos+1:], src[srcPos+1:]) {
					return true
				} else {
					srcPos++
				}
			}
			return false
		case '?':
			srcPos++
			break
		case '[':
			var not, match, closePat bool
			patPos++
			// '[' must match a ']' character, otherwise it's wrong pattern and return false
			if patPos == patLen {
				return false
			}
			if pattern[patPos] == '^' {
				not = true
				patPos++
			}
			for patPos < patLen {
				if pattern[patPos] == '\\' {
					patPos++
					if patPos == patLen { // pattern syntax error
						return false
					}
					if pattern[patPos] == src[srcPos] {
						match = true
					}
				} else if pattern[patPos] == ']' {
					closePat = true
					break
				} else if pattern[patPos] == '-' {
					if patPos+1 == patLen || pattern[patPos+1] == ']' { //  wrong pattern syntax
						return false
					}
					start, end := pattern[patPos-1], pattern[patPos+1]
					if src[srcPos] >= start && src[srcPos] <= end {
						match = true
					}
					patPos++
				} else {
					if pattern[patPos] == src[srcPos] {
						match = true
					}
				}
				patPos++
			}
			if !closePat {
				return false
			}
			if not {
				match = !match
			}
			if !match {
				return false
			}
			srcPos++
			break
		case '\\':
			//	escape special character in pattern and fall through to default to handle
			if patPos+1 < patLen {
				patPos++
			} else {
				return false
			}
			//	fall into default
		default:
			if pattern[patPos] != src[srcPos] {
				return false
			}
			srcPos++
			break
		}
		patPos++
		// When src has been consumed, pattern must be consumed to the end or only contains '*' in last
		if srcPos >= srcLen {
			for patPos < patLen && pattern[patPos] == '*' {
				patPos++
			}
			break
		}
	}
	return patPos == patLen && srcPos == srcLen
}
