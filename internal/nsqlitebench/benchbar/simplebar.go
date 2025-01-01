// Package benchbar provides a really simple progress bar for the benchmarking
// process.
package benchbar

import (
	"github.com/schollz/progressbar/v3"
)

type progressBar struct {
	pb          *progressbar.ProgressBar
	description string
	maxItems    int
}

func NewBar(description string, maxItems int) *progressBar {
	pb := progressbar.Default(int64(maxItems), description)
	_ = pb.Set(0)

	return &progressBar{
		pb:          pb,
		description: description,
		maxItems:    maxItems,
	}
}

func (p *progressBar) Inc() {
	_ = p.pb.Add(1)
}

func (p *progressBar) Finish() {
	_ = p.pb.Finish()
	_ = p.pb.Close()
}
