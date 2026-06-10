// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const (
	testUnixAbsRef    = "/abs/models.json"
	testWinDrivePath  = "c:/dir/x.json"
	testBeneathBaseID = "/base"
)

func TestAbsoluteLocalRefPath(t *testing.T) {
	tests := []struct {
		ref      string
		wantPath string
		wantOK   bool
	}{
		{"#/definitions/Pet", "", false},                // fragment-only
		{"./models.json#/definitions/Model", "", false}, // relative
		{"models.json", "", false},                      // relative, no leading dot
		{testUnixAbsRef, testUnixAbsRef, true},          // unix absolute
		{"file://" + testUnixAbsRef, testUnixAbsRef, true},
		{"file:///D:/a/x.json", "d:/a/x.json", true},             // windows drive, canonical 3-slash file URL
		{"file://D:/a/x.json", "d:/a/x.json", true},              // windows drive lands in URL host (invalid hybrid)
		{"file://host/share/x.json", "/host/share/x.json", true}, // UNC: host kept, local-dubious
		{`C:\dir\x.json`, testWinDrivePath, true},                // windows drive, backslashes
		{"C:/dir/x.json", testWinDrivePath, true},                // windows drive, forward slashes
		{"http://example.com/a.json#/x", "", false},              // remote
		{"https://other.com/b.json", "", false},                  // remote
		{"//proto.relative/c.json", "", false},                   // protocol-relative => remote, not local
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			r := spec.MustCreateRef(tc.ref)
			got, ok := absoluteLocalRefPath(r, r.GetURL())
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantPath, got)
		})
	}
}

// TestWindowsDriveRefFormsConverge guards the Windows nuance that cannot be exercised on a
// non-Windows runner: the valid bare-drive (C:\, C:/) and canonical file:///C:/ forms, plus the
// invalid-but-tolerated hybrid file://C:/ form (drive lands in the URL host), must all normalize
// to the SAME drive path - and that path must compare correctly against a base derived from
// SpecFilePath (which carries no leading slash). Regression guard for the leading-slash mismatch
// that previously made a legit canonical ref beneath the base spuriously warn.
func TestWindowsDriveRefFormsConverge(t *testing.T) {
	const base = "c:/specdir"
	const want = "c:/specdir/sub/models.json"
	for _, ref := range []string{
		"file:///C:/specdir/sub/models.json", // canonical 3-slash file URL (valid)
		"file://C:/specdir/sub/models.json",  // hybrid 2-slash, drive in host (invalid, tolerated)
		"C:/specdir/sub/models.json",         // bare drive, forward slashes (valid)
		`C:\specdir\sub\models.json`,         // bare drive, backslashes (valid)
	} {
		t.Run(ref, func(t *testing.T) {
			r := spec.MustCreateRef(ref)
			got, ok := absoluteLocalRefPath(r, r.GetURL())
			require.True(t, ok)
			assert.Equal(t, want, got, "all drive forms must normalize identically")
			assert.True(t, isBeneathBase(got, base), "%q should be beneath %q", got, base)
		})
	}
}

func TestRemoteRefHost(t *testing.T) {
	tests := []struct {
		ref      string
		wantHost string
	}{
		{"http://example.com/a.json", "example.com"},
		{"https://other.com/b.json", "other.com"},
		{"//proto.relative/c.json", "proto.relative"},
		{"/abs/models.json", ""},
		{"#/definitions/Pet", ""},
		{"file:///abs/models.json", ""}, // file scheme is local, not a remote host
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			r := spec.MustCreateRef(tc.ref)
			assert.Equal(t, tc.wantHost, remoteRefHost(r.GetURL()))
		})
	}
}

func TestCleanRefPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/abs/x.json", "/abs/x.json"},
		{`C:\dir\x.json`, testWinDrivePath},
		{testWinDrivePath, testWinDrivePath},
		{"/C:/dir/x.json", "c:/dir/x.json"}, // canonical file:// URL drive: leading slash dropped
		{"/abs/../y.json", "/y.json"},
		{"/Case/Sensitive", "/Case/Sensitive"}, // unix case preserved
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, cleanRefPath(tc.in))
		})
	}
}

func TestIsBeneathBase(t *testing.T) {
	tests := []struct {
		target, base string
		want         bool
	}{
		{testBeneathBaseID + "/sub/x.json", testBeneathBaseID, true},
		{testBeneathBaseID, testBeneathBaseID, true},
		{testBeneathBaseID + "/x.json", testBeneathBaseID, true},
		{"/other/x.json", testBeneathBaseID, false},
		{"/baseball/x.json", testBeneathBaseID, false}, // prefix but not a path boundary
		{"/x.json", testBeneathBaseID, false},
		{testWinDrivePath, "c:/dir", true},
		{"c:/other/x.json", "c:/dir", false},
		{"", testBeneathBaseID, false},        // no target
		{testBeneathBaseID + "/x", "", false}, // no base
	}
	for _, tc := range tests {
		t.Run(tc.target+"|"+tc.base, func(t *testing.T) {
			assert.Equal(t, tc.want, isBeneathBase(tc.target, tc.base))
		})
	}
}

// dubiousValidatorFromJSON builds a SpecValidator wired with an analyzer over an in-memory spec
// (SpecFilePath is empty, so absolute-local refs have no base and are treated as dubious).
func dubiousValidatorFromJSON(t *testing.T, doc string) *SpecValidator {
	t.Helper()
	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)
	s := NewSpecValidator(d.Schema(), strfmt.Default)
	s.spec = d
	s.analyzer = analysis.New(d.Spec())
	return s
}

func warningMessages(res *Result) []string {
	msgs := make([]string, 0, len(res.Warnings))
	for _, w := range res.Warnings {
		msgs = append(msgs, w.Error())
	}
	return msgs
}

func TestValidateDubiousRefs_MultipleHosts(t *testing.T) {
	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "http://host-one.example/a.json"},
			"B": {"$ref": "https://host-two.example/b.json"}
		}
	}`
	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	msgs := warningMessages(res)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "distinct remote hosts")
	assert.Contains(t, msgs[0], "host-one.example")
	assert.Contains(t, msgs[0], "host-two.example")
}

func TestValidateDubiousRefs_SingleHostNoWarning(t *testing.T) {
	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "https://only-host.example/a.json"},
			"B": {"$ref": "https://only-host.example/b.json"}
		}
	}`
	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	assert.Empty(t, warningMessages(res), "a single consistent remote host must not warn")
}

func TestValidateDubiousRefs_AbsoluteLocalNoBase(t *testing.T) {
	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "file:///etc/passwd"}
		}
	}`
	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	msgs := warningMessages(res)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "escapes the spec's base path")
	assert.Contains(t, msgs[0], "/etc/passwd")
}

func TestValidateDubiousRefs_FragmentAndRelativeNoWarning(t *testing.T) {
	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "#/definitions/B"},
			"B": {"type": "object"},
			"C": {"$ref": "./models.json#/definitions/X"}
		}
	}`
	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	assert.Empty(t, warningMessages(res), "fragment-only and relative refs must not warn")
}

// TestValidateDubiousRefs_AbsoluteBeneathBase exercises Fred's critical nuance end-to-end:
// an absolute local ref that stays BENEATH the spec's base path is legitimate (flatten/expand
// introduces such anchors for cyclical $refs) and must NOT warn, whereas one that escapes does.
func TestValidateDubiousRefs_AbsoluteBeneathBase(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.json")
	beneath := filepath.Join(dir, "models.json")               // absolute, beneath base
	escape := filepath.Join(filepath.Dir(dir), "outside.json") // absolute, escapes base

	toFileRef := func(p string) string {
		return "file://" + filepath.ToSlash(p)
	}

	doc := fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"Beneath": {"$ref": %q},
			"Escape": {"$ref": %q}
		}
	}`, toFileRef(beneath), toFileRef(escape))

	require.NoError(t, os.WriteFile(specPath, []byte(doc), 0o600))

	d, err := loads.Spec(specPath)
	require.NoError(t, err)
	require.NotEmpty(t, d.SpecFilePath())

	s := NewSpecValidator(d.Schema(), strfmt.Default)
	s.spec = d
	s.analyzer = analysis.New(d.Spec())

	msgs := warningMessages(s.validateDubiousRefs())
	require.Len(t, msgs, 1, "only the escaping ref should warn, not the one beneath base")
	assert.Contains(t, msgs[0], "outside.json")
	for _, m := range msgs {
		assert.NotContains(t, m, "models.json", "absolute ref beneath base must not warn")
	}
}

func TestLocalBaseDir(t *testing.T) {
	t.Run("empty spec path yields no base", func(t *testing.T) {
		d, err := loads.Analyzed(json.RawMessage(`{"swagger":"2.0","info":{"title":"t","version":"1"},"paths":{}}`), "")
		require.NoError(t, err)
		s := &SpecValidator{spec: d}
		_, ok := s.localBaseDir()
		assert.False(t, ok)
	})

	t.Run("local file path yields its directory", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.json")
		require.NoError(t, os.WriteFile(specPath, []byte(`{"swagger":"2.0","info":{"title":"t","version":"1"},"paths":{}}`), 0o600))
		d, err := loads.Spec(specPath)
		require.NoError(t, err)
		s := &SpecValidator{spec: d}
		base, ok := s.localBaseDir()
		require.True(t, ok)
		assert.True(t, strings.HasSuffix(base, cleanRefPath(dir)), "base %q should end with %q", base, cleanRefPath(dir))
	})
}
