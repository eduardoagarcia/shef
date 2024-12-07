package stages

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/term"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

type InteractiveCommandStage struct{}

func init() {
	Register("interactive_command", func() StageRunner {
		return &InteractiveCommandStage{}
	})
}

func (c *InteractiveCommandStage) Run(input string, config map[string]interface{}) (string, error) {
	command, ok := config["command"].(string)
	if !ok {
		return "", fmt.Errorf("command configuration missing")
	}

	cmd := exec.Command("bash", "-c", strings.ReplaceAll(command, "{{input}}", input))
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %v", err)
	}
	defer ptmx.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	pty.InheritSize(os.Stdin, ptmx)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH
	defer signal.Stop(ch)

	go io.Copy(ptmx, os.Stdin)
	go io.Copy(os.Stdout, ptmx)

	return "", cmd.Wait()
}
