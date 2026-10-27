package minimax

import "strings"

// bearerScheme is the authentication scheme MiniMax expects in front of
// the API key.
const bearerScheme = "bearer"

// invisible are the characters that a copy from a web page, a PDF or a
// chat message leaves behind. None of them belong in an API key, but any
// of them breaks the Authorization header.
var invisible = strings.NewReplacer(
	"\u00a0", "", // non-breaking space
	"\u2007", "", // figure space
	"\u202f", "", // narrow no-break space
	"\u200b", "", // zero width space
	"\u200c", "", // zero width non-joiner
	"\u200d", "", // zero width joiner
	"\ufeff", "", // byte order mark
	"\r", "",
	"\n", "",
	"\t", "",
)

// wrappers are the quote and bracket pairs that survive a copy from
// documentation, for example `"sk-..."` or `[sk-...]`.
var wrappers = [][2]string{
	{`"`, `"`},
	{`'`, `'`},
	{"`", "`"},
	{"[", "]"},
	{"<", ">"},
}

// SanitizeAPIKey normalises a MiniMax API key.
//
// Keys are copied from the MiniMax dashboard, from documentation and from
// chat messages, which drags along whitespace, invisible characters and
// sometimes the surrounding quotes or the whole "Bearer ..." header value.
// MiniMax rejects every one of those with status 1004, "login fail: Please
// carry the API secret key in the 'Authorization' field of the request
// header", which reads like a wrong key rather than a formatting problem.
//
// The key is normalised before it is stored and again before it is sent,
// so that a key saved by an older build keeps working.
func SanitizeAPIKey(raw string) string {
	key := invisible.Replace(raw)

	// Peel the wrappers inwards: a value pasted from documentation can be
	// quoted and prefixed at once, such as `"Bearer eyJ..."`.
	for range 4 {
		before := key
		key = strings.TrimSpace(key)
		key = trimBearerPrefix(key)
		key = trimWrapper(key)
		if key == before {
			break
		}
	}

	return strings.TrimSpace(key)
}

// trimBearerPrefix removes a leading bearer scheme, in any case.
//
// The scheme is only stripped when nothing or a separator follows it, so
// that a key which merely begins with those letters is left alone.
func trimBearerPrefix(key string) string {
	if len(key) < len(bearerScheme) || !strings.EqualFold(key[:len(bearerScheme)], bearerScheme) {
		return key
	}

	rest := key[len(bearerScheme):]
	if rest == "" || strings.HasPrefix(rest, " ") {
		return rest
	}
	return key
}

// trimWrapper removes one pair of matching surrounding quotes or brackets.
func trimWrapper(key string) string {
	for _, pair := range wrappers {
		open, close := pair[0], pair[1]
		if len(key) >= len(open)+len(close) && strings.HasPrefix(key, open) && strings.HasSuffix(key, close) {
			return key[len(open) : len(key)-len(close)]
		}
	}
	return key
}
