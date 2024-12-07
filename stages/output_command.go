package stages

import (
	"fmt"
	"github.com/creack/pty"
	"io"
	"os"
	"os/exec"
	"strings"
)

type OutputCommandStage struct{}

func init() {
	Register("output_command", func() StageRunner {
		return &OutputCommandStage{}
	})
}

func (c *OutputCommandStage) Run(input string, config map[string]interface{}) (string, error) {
	command, ok := config["command"].(string)
	if !ok {
		return "", fmt.Errorf("command configuration missing")
	}

	command = strings.ReplaceAll(command, "{{input}}", input)
	cmd := exec.Command("bash", "-c", command)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %v", err)
	}
	defer ptmx.Close()

	var output strings.Builder
	go func() {
		mw := io.MultiWriter(os.Stdout, &output)
		io.Copy(mw, ptmx)
	}()

	return output.String(), cmd.Wait()
}
