// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import "github.com/go-openapi/spec"

type schemaCompositionFrame struct {
	schema *spec.Schema
	path   pathSegments
	depth  int
}

func firstSchemaCompositionDepthExceeded(schema *spec.Schema, at pathSegments, maxDepth int) (pathSegments, bool) {
	if schema == nil {
		return nil, false
	}

	stack := []schemaCompositionFrame{{
		schema: schema,
		path:   at,
		depth:  0,
	}}

	for len(stack) > 0 {
		last := len(stack) - 1
		frame := stack[last]
		stack = stack[:last]

		if frame.depth > maxDepth {
			return frame.path, true
		}

		if frame.schema.Not != nil {
			stack = append(stack, schemaCompositionFrame{
				schema: frame.schema.Not,
				path:   frame.path.child(jsonNot),
				depth:  frame.depth + 1,
			})
		}

		for i := len(frame.schema.OneOf) - 1; i >= 0; i-- {
			stack = append(stack, schemaCompositionFrame{
				schema: &frame.schema.OneOf[i],
				path:   frame.path.child(jsonOneOf).item(i),
				depth:  frame.depth + 1,
			})
		}

		for i := len(frame.schema.AnyOf) - 1; i >= 0; i-- {
			stack = append(stack, schemaCompositionFrame{
				schema: &frame.schema.AnyOf[i],
				path:   frame.path.child(jsonAnyOf).item(i),
				depth:  frame.depth + 1,
			})
		}

		for i := len(frame.schema.AllOf) - 1; i >= 0; i-- {
			stack = append(stack, schemaCompositionFrame{
				schema: &frame.schema.AllOf[i],
				path:   frame.path.child(jsonAllOf).item(i),
				depth:  frame.depth + 1,
			})
		}
	}

	return nil, false
}
