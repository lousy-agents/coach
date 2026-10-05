package engine

import (
	"github.com/odvcencio/gotreesitter"
)

func (n *gtsNode) Parent() Node {
	p := n.n.Parent()
	if p == nil {
		return nil
	}
	return &gtsNode{n: p, lang: n.lang}
}
func (l *gtsLanguage) lang() *gotreesitter.Language {
	lang := l.entry.Language()
	if l.wantsForest {
		l.forestOnce.Do(func() {

			lang.WantsForest = true
		})
	}
	return lang
}
