// removebell removes the bell sound everytime a line is changed with promptui.
package removebell

import (
	"io"
	"os"

	"github.com/chzyer/readline"
)

func init() {
	readline.Stdout = &noReadlineBells{
		underlying: os.Stdout,
	}
}

var _ io.WriteCloser = &noReadlineBells{}

// noReadlineBells strips out the bells sent by readline. Without this, every
// time we switch lines a bell rung (see
// https://github.com/manifoldco/promptui/issues/49).
type noReadlineBells struct {
	underlying io.WriteCloser
}

func (n *noReadlineBells) Write(p []byte) (int, error) {
	// When readline writes a bell, it writes it as a lone character, so we only
	// need to check if the entire buffer is length one.
	if len(p) == 1 && p[0] == readline.CharBell {
		return 1, nil
	}
	return n.underlying.Write(p)
}

func (n *noReadlineBells) Close() error {
	return n.underlying.Close()
}
