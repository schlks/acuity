package web

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

type folderTree struct {
	once sync.Once
	path string
	subFolders []*SubFolder
}

type folderTreeCache struct {
	mu sync.Mutex
	trees map[int]*folderTree
}

func newFolderTreeCache() *folderTreeCache {
	return &folderTreeCache{trees: make(map[int]*folderTree)}
}

func (c *folderTreeCache) get(id int, path string) []*SubFolder {
	c.mu.Lock()
	t, ok := c.trees[id]
	if !ok || t.path != path {
		t = &folderTree{path: path}
		c.trees[id] = t
	}
	c.mu.Unlock()

	t.once.Do(func() { t.subFolders = walkSubFolders(path) })
	return t.subFolders
}

func (c *folderTreeCache) invalidate(ids ...int) {
	c.mu.Lock()
	for _, id := range ids {
		delete(c.trees, id)
	}
	c.mu.Unlock()
}

func (c *folderTreeCache) invalidateAll() {
	c.mu.Lock()
	c.trees = make(map[int]*folderTree)
	c.mu.Unlock()
}

func walkSubFolders(root string) []*SubFolder {
	var subFolders []*SubFolder
	nodeMap := make(map[string]*SubFolder)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root || !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		node := &SubFolder{Name: d.Name(), Path: path}
		nodeMap[path] = node

		parentPath := filepath.Dir(path)
		if parentPath == root {
			subFolders = append(subFolders, node)
		} else if parentNode, ok := nodeMap[parentPath]; ok {
			parentNode.SubFolders = append(parentNode.SubFolders, node)
		}
		return nil
	})

	return subFolders
}
