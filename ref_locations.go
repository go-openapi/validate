// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

// refLocations indexes the $ref values declared by a raw document, telling
// where each one sits.
//
// The analyzer keeps the same index — its reference map is keyed by document
// location — but hands over only the values, so the locations are recovered
// here. Finding them does not require interpreting a reference: it is enough
// to spot the "$ref" members while walking the document.
//
// The result is best effort. A reference may be declared in several places,
// and only one location is kept; example values are skipped, but any other
// data that happens to hold a "$ref" string could still be mistaken for a
// declaration. A wrong answer costs a misleading pointer, never a wrong
// verdict, since nothing else is decided from it.
type refLocations map[string]pathSegments

// newRefLocations indexes every reference declared by a raw document.
func newRefLocations(document any) refLocations {
	locations := make(refLocations)
	locations.collect(document, rootPath())

	return locations
}

// at returns where a reference is declared, or the document root when the
// reference was not found.
func (l refLocations) at(ref string) pathSegments {
	return l[ref]
}

func (l refLocations) collect(node any, at pathSegments) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			if key == jsonRef {
				if ref, isString := value.(string); isString && ref != "" {
					l.keep(ref, at)
				}

				continue
			}

			if isValueMember(key, at) {
				// plain data: a "$ref" down there declares nothing
				continue
			}

			l.collect(value, at.child(key))
		}
	case []any:
		for i, value := range typed {
			l.collect(value, at.item(i))
		}
	}
}

// isValueMember tells if a member holds plain data rather than schemas.
//
// Examples and defaults are values, so a "$ref" member inside them declares
// nothing. The exception is "default" under "responses", which names a
// response rather than holding a value, and may legitimately be a $ref.
func isValueMember(key string, at pathSegments) bool {
	switch key {
	case swaggerExample, swaggerExamples:
		return true
	case jsonDefault:
		return at.last() != swaggerResponses
	default:
		return false
	}
}

// keep records a location for a reference, settling ties by the smallest
// pointer so that the answer does not depend on map iteration order.
func (l refLocations) keep(ref string, at pathSegments) {
	known, isKnown := l[ref]
	if isKnown && known.pointer() <= at.pointer() {
		return
	}

	l[ref] = at
}
