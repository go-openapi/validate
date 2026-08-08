// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strconv"
	"strings"

	"github.com/go-openapi/jsonpointer"
)

// pathSegments is the location of a validated value inside a document,
// held as an ordered list of unescaped JSON pointer reference tokens.
//
// Validators build a location by appending tokens as they descend into
// properties and array items, then render it only when they report an error.
// Keeping the tokens apart until then is what makes it possible to produce a
// valid [RFC 6901] JSON pointer: a token is escaped when it is rendered, and
// the separator can never be confused with a token that contains one.
//
// The zero value is the location of the document root.
//
// [RFC 6901]: https://datatracker.ietf.org/doc/html/rfc6901
type pathSegments []string

// newPathSegments builds a location from a list of unescaped tokens.
func newPathSegments(tokens ...string) pathSegments {
	if len(tokens) == 0 {
		return nil
	}

	return pathSegments(tokens)
}

// rootPath is the location of the document root.
func rootPath() pathSegments { return nil }

// String implements [fmt.Stringer] with the legacy dotted notation, so that a
// location interpolated into a message reads as it always has.
func (p pathSegments) String() string { return p.dotted() }

// child returns the location of a named member of the value at p.
//
// The receiver is never modified: sibling children may be derived from the
// same parent without aliasing one another.
func (p pathSegments) child(token string) pathSegments {
	child := make(pathSegments, len(p)+1)
	copy(child, p)
	child[len(p)] = token

	return child
}

// children returns the location of a chain of named members below p.
func (p pathSegments) children(tokens ...string) pathSegments {
	child := make(pathSegments, len(p)+len(tokens))
	copy(child, p)
	copy(child[len(p):], tokens)

	return child
}

// item returns the location of the index'th element of the array at p.
func (p pathSegments) item(index int) pathSegments {
	return p.child(strconv.Itoa(index))
}

// isEmpty tells if p locates the document root.
func (p pathSegments) isEmpty() bool { return len(p) == 0 }

// last returns the trailing token, or an empty string at the document root.
func (p pathSegments) last() string {
	if len(p) == 0 {
		return ""
	}

	return p[len(p)-1]
}

// beforeLast returns the token before the trailing one, or an empty string
// when p holds fewer than two tokens.
func (p pathSegments) beforeLast() string {
	const beforeLast = 2
	if len(p) < beforeLast {
		return ""
	}

	return p[len(p)-beforeLast]
}

// trimIndexes returns p without its trailing array index tokens.
//
// It answers "what is this value inside of", disregarding how deep into an
// array it sits: the items of an example are still an example.
func (p pathSegments) trimIndexes() pathSegments {
	end := len(p)
	for end > 0 && isIndexToken(p[end-1]) {
		end--
	}

	return p[:end]
}

// isIndexToken tells if a token addresses an array element rather than a member.
func isIndexToken(token string) bool {
	if token == "" {
		return false
	}

	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// hasSuffix tells if p ends with the given sequence of tokens.
func (p pathSegments) hasSuffix(suffix pathSegments) bool {
	if len(suffix) > len(p) {
		return false
	}

	offset := len(p) - len(suffix)
	for i, token := range suffix {
		if p[offset+i] != token {
			return false
		}
	}

	return true
}

// dotted renders the location in the legacy dot-separated notation, e.g.
// "definitions.Pet.friends.0.name".
//
// Tokens are emitted verbatim: a token containing a dot is indistinguishable
// from a separator. This notation is kept because it is what surfaces as the
// name of a validation error, and API consumers of go-swagger servers see it.
// Use [pathSegments.pointer] whenever the location needs to be unambiguous.
func (p pathSegments) dotted() string {
	return strings.Join(p, ".")
}

// pointer renders the location as an RFC 6901 JSON pointer, e.g.
// "/definitions/Pet/friends/0/name". The document root renders as "".
func (p pathSegments) pointer() string {
	if len(p) == 0 {
		return ""
	}

	var w strings.Builder
	for _, token := range p {
		w.WriteByte('/')
		w.WriteString(jsonpointer.Escape(token))
	}

	return w.String()
}
