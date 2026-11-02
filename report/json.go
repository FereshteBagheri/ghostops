package report

import (
	"encoding/json"
	"io"
)

type JSON struct{}

func (JSON) Render(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
