// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/require"
	yaml "go.yaml.in/yaml/v3"
)

// TestGenExpectations rewrites testdata/validation/expected_messages.yaml from what the
// validators actually produce.
//
// It edits the yaml tree in place rather than re-marshalling it, so the comments the file
// carries survive: an expectation that still matches keeps its node, its spelling and its
// comment, one that matches nothing is deleted, and an actual message nothing covers is
// appended.
//
// Run it with:
//
//	SWAGGER_GEN=1 go test -run TestGenExpectations . -args -enable-long
//
// then read the diff: a message that changed is a change in what a user is told, and each one
// needs a look before it is committed.
func TestGenExpectations(t *testing.T) {
	if os.Getenv("SWAGGER_GEN") == "" {
		t.Skip("set SWAGGER_GEN=1 to rewrite the expectations")
	}

	cfg := filepath.Join("testdata", "validation", "expected_messages.yaml")
	tested := loadTestConfig(t, cfg)

	raw, err := os.ReadFile(cfg)
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	root := doc.Content[0]

	err = filepath.Walk(filepath.Join("testdata", "validation"), func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		fixture, found := tested.Get(info.Name())
		if !found || fixture.ExpectedLoadError {
			return nil
		}

		spec, err := loads.Spec(path)
		if err != nil {
			t.Logf("SKIP (load error) %s: %v", path, err)
			return nil
		}

		errsStop, warnStop := runValidator(spec, false)
		errsCont, warnCont := runValidator(spec, true)

		node := fixtureNode(root, info.Name())
		require.NotNil(t, node, "no node for %s", info.Name())
		patch(t, node, "expectedMessages", fixture.ExpectedMessages, errsStop, errsCont)
		patch(t, node, "expectedWarnings", fixture.ExpectedWarnings, warnStop, warnCont)

		return nil
	})
	require.NoError(t, err)

	var out strings.Builder
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	require.NoError(t, enc.Encode(&doc))
	require.NoError(t, enc.Close())
	require.NoError(t, os.WriteFile(cfg, []byte(out.String()), 0o600))
	t.Logf("rewrote %s", cfg)
}

func fixtureNode(root *yaml.Node, name string) *yaml.Node {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == name {
			return root.Content[i+1]
		}
	}

	return nil
}

func runValidator(doc *loads.Document, continueOnErrors bool) ([]string, []string) {
	v := NewSpecValidator(doc.Schema(), strfmt.Default)
	v.SetContinueOnErrors(continueOnErrors)
	res, warn := v.Validate(doc)

	msgs := func(r *Result) []string {
		out := make([]string, 0, len(r.Errors))
		for _, e := range r.Errors {
			out = append(out, e.Error())
		}

		return out
	}

	return msgs(res), msgs(warn)
}

// patch rewrites one expectation list of one fixture.
func patch(t *testing.T, fixture *yaml.Node, key string, expected []ExpectedMessage, stop, cont []string) {
	t.Helper()

	var seq *yaml.Node
	for i := 0; i+1 < len(fixture.Content); i += 2 {
		if fixture.Content[i].Value == key {
			seq = fixture.Content[i+1]
		}
	}
	require.NotNil(t, seq, "no %s in fixture", key)
	require.EqualT(t, len(expected), len(seq.Content), key+": node count and decoded count disagree")

	covered := make(map[string]bool, len(cont))
	kept := make([]*yaml.Node, 0, len(expected))

	for i, e := range expected {
		var hitsStop, hitsCont bool
		for _, a := range stop {
			if matches(e, a) {
				hitsStop = true
			}
		}
		for _, a := range cont {
			if matches(e, a) {
				hitsCont = true
				covered[a] = true
			}
		}
		if !hitsStop && !hitsCont {
			continue // stale: matches nothing any more
		}
		setBool(seq.Content[i], "withContinueOnErrors", !hitsStop)
		kept = append(kept, seq.Content[i])
	}

	inStop := make(map[string]bool, len(stop))
	for _, s := range stop {
		inStop[s] = true
	}
	for _, a := range cont {
		if covered[a] {
			continue
		}
		kept = append(kept, messageNode(a, !inStop[a]))
	}

	seq.Content = kept
	if len(kept) == 0 {
		seq.Style = yaml.FlowStyle
	}
}

func setBool(item *yaml.Node, key string, value bool) {
	for i := 0; i+1 < len(item.Content); i += 2 {
		if item.Content[i].Value == key {
			item.Content[i+1].Value = boolString(value)
			item.Content[i+1].Tag = "!!bool"

			return
		}
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

func messageNode(message string, withContinueOnErrors bool) *yaml.Node {
	scalar := func(value, tag string, style yaml.Style) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value, Style: style}
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			scalar("message", "!!str", 0), scalar(message, "!!str", yaml.SingleQuotedStyle),
			scalar("withContinueOnErrors", "!!str", 0), scalar(boolString(withContinueOnErrors), "!!bool", 0),
			scalar("isRegexp", "!!str", 0), scalar("false", "!!bool", 0),
		},
	}
}

func matches(e ExpectedMessage, actual string) bool {
	if e.IsRegexp {
		ok, _ := regexp.MatchString(e.Message, actual)

		return ok
	}

	return strings.Contains(actual, e.Message)
}
