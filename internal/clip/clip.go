package clip

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Copy places text on the system clipboard.
// It prefers native helpers when present, then falls back to OSC 52
// (works over SSH and in many terminal emulators / tmux).
func Copy(text string) error {
	if err := copyWithHelper(text); err == nil {
		return nil
	}
	return CopyOSC52(text)
}

// CopyOSC52 writes an OSC 52 sequence to the controlling terminal.
func CopyOSC52(text string) error {
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\x1b]52;c;%s\x07", enc)
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// fall back to stdout (PTY sessions)
		if _, werr := os.Stdout.WriteString(seq); werr != nil {
			return fmt.Errorf("osc52: %w", werr)
		}
		return nil
	}
	defer f.Close()
	if _, err := f.WriteString(seq); err != nil {
		return err
	}
	return nil
}

func copyWithHelper(text string) error {
	type helper struct {
		bin  string
		args []string
	}
	helpers := []helper{
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
		{"pbcopy", nil},
	}
	for _, h := range helpers {
		path, err := exec.LookPath(h.bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, h.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			continue
		}
		return nil
	}
	return fmt.Errorf("no clipboard helper available")
}
