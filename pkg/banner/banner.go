package banner

import (
	_ "embed"
	"fmt"
	"github.com/charmbracelet/ssh"
)

//go:embed banner.txt
var banner string

func CometWelcome(ctx ssh.Context) string {
	return fmt.Sprintf(banner, ctx.User())
}
