package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

func (g *GitBackend) GetPage(title string) (*Page, error) {
	return g.GetPageAt(title, "HEAD")
}

func (g *GitBackend) GetFile(name string) (io.ReadCloser, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, gitPath, err := g.resolvePath(g.dir, name)
	_, b, err := g.pathAtRevision(gitPath, revision)
	if err != nil {
		return nil, err
	}

	return b
}

func (g *GitBackend) GetConfig(name string) ([]byte, error) {
	filePath := filepath.Join(g.dir, ".wiki", fmt.Sprintf("%s.json.enc", name))
	return os.ReadFile(filePath)
}
