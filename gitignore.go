package systheme

import (
	"github.com/gofuego/fuego-formats/formatkit"
	"github.com/gofuego/fuego/core"
)

// gitignoreType marks the .gitignore page. It is home-page furniture, not a
// format: ComposeHome lifts its content into the landing page's collapsible
// and skips the page, so it never renders standalone.
const gitignoreType = "gitignore"

func gitignoreParser() core.Parser {
	return formatkit.NewParser(gitignoreType,
		func(payload []byte) (core.Envelope, []core.Node, error) {
			return core.Envelope{}, []core.Node{{
				Type:    gitignoreType,
				Content: string(payload),
			}}, nil
		},
		formatkit.WithDefaultPatterns(".gitignore"))
}
