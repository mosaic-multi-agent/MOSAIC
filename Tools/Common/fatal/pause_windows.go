//go:build windows

package fatal

import (
	"io"
	"os"
	"syscall"
)

var procFlushConsoleInputBuffer = syscall.NewLazyDLL("kernel32.dll").NewProc("FlushConsoleInputBuffer")

// discardPendingInput drops keystrokes already buffered on the real console input,
// so keys typed before the prompt do not end the pause at once.
func discardPendingInput(in io.Reader) {
	f, ok := in.(*os.File)
	if !ok || f != os.Stdin {
		return
	}
	_, _, _ = procFlushConsoleInputBuffer.Call(f.Fd())
}
