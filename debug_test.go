// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"os"
	"sync"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

var (
	logMutex = &sync.Mutex{}
)

func TestDebug(t *testing.T) {
	if !enableLongTests {
		skipNotify(t)
		t.SkipNow()
	}

	// standard lib t.TempDir() is still subject to an issue https://github.com/golang/go/issues/71544
	// Hence: usetesting linter disabled
	tmpFile, _ := os.CreateTemp("", "debug-test") //nolint:usetesting
	tmpName := tmpFile.Name()
	defer func() {
		Debug = false
		// mutex for -race
		logMutex.Unlock()
		os.Remove(tmpName)
	}()

	// mutex for -race
	logMutex.Lock()
	Debug = true
	debugOptions()
	defer func() {
		validateLogger.SetOutput(os.Stdout)
	}()

	validateLogger.SetOutput(tmpFile)

	debugLog("A debug")
	Debug = false
	tmpFile.Close()

	flushed, _ := os.Open(tmpName)
	buf := make([]byte, 500)
	_, _ = flushed.Read(buf)
	validateLogger.SetOutput(os.Stdout)
	assert.Contains(t, string(buf), "A debug")
}
