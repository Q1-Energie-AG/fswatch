package fswatch

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const TestfolderPath = "./testfolder"

// setup assures that the testing directory exists and is empty
func setup(t *testing.T) {
	if _, err := os.Stat(TestfolderPath); os.IsExist(err) {
		err := os.RemoveAll(TestfolderPath)
		assert.Nil(t, err)
	}

	err := os.Mkdir(TestfolderPath, os.ModePerm)
	assert.Nil(t, err)

}

// teardown deletes the testing directory after the tests
func teardown(t *testing.T) {
	err := os.RemoveAll(TestfolderPath)
	assert.Nil(t, err)
}

func getDebounceMapCount(w *Watcher) int {
	w.debounceMapMu.Lock()
	defer w.debounceMapMu.Unlock()
	return len(w.debounceMap)
}

func TestWatcher(t *testing.T) {

	// Setup + Teardown
	setup(t)
	defer teardown(t)

	// Create new watcher
	w, err := NewWatcher(time.Second)
	assert.Nil(t, err)

	assert.Equal(t, 0, getDebounceMapCount(w))

	// Add a folder to the watcher
	err = w.Add(TestfolderPath)
	assert.Nil(t, err)

	// Create file and write to it
	f, err := os.Create(TestfolderPath + "/test.txt")
	assert.Nil(t, err)

	_, err = f.WriteString("test\n")
	assert.Nil(t, err)

	// Give time to recognize the CREATE / WRITE operation
	time.Sleep(time.Millisecond * 100)

	assert.Equal(t, 1, getDebounceMapCount(w))

	// Wait for the event or a timeout
	select {
	case <-w.Events:
		assert.Equal(t, 0, getDebounceMapCount(w))
	case <-time.After(time.Second * 2):
		assert.Fail(t, "no event received")
	}

	// Cleanup the watcher
	err = w.Remove(TestfolderPath)
	assert.Nil(t, err)

	err = w.Close()
	assert.Nil(t, err)

}
