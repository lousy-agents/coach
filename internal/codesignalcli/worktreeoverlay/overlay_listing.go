package worktreeoverlay

import (
	"io/fs"
	"path"
	"sort"
)

// index records, for every overlaid file, the directory entry it contributes
// to its parent and the directories that must exist to hold it.
func (o *overlayFS) index() {
	o.dirs = map[string]bool{".": true}
	o.added = map[string]map[string]fs.DirEntry{}
	for name, data := range o.files {
		o.addEntry(path.Dir(name), memInfo{name: path.Base(name), size: int64(len(data))})
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			o.dirs[dir] = true
			o.addEntry(path.Dir(dir), memInfo{name: path.Base(dir), dir: true})
		}
	}
}

func (o *overlayFS) addEntry(parent string, info memInfo) {
	if o.added[parent] == nil {
		o.added[parent] = map[string]fs.DirEntry{}
	}
	o.added[parent][info.name] = fs.FileInfoToDirEntry(info)
}

// mergeEntries lays the overlay's entries over a base directory listing:
// hidden paths drop out, and overlaid files replace or join the listing.
func (o *overlayFS) mergeEntries(dir string, base []fs.DirEntry) []fs.DirEntry {
	byName := make(map[string]fs.DirEntry, len(base)+len(o.added[dir]))
	for _, entry := range base {
		if !o.hidden[join(dir, entry.Name())] {
			byName[entry.Name()] = entry
		}
	}
	for name, entry := range o.added[dir] {
		byName[name] = entry
	}
	merged := make([]fs.DirEntry, 0, len(byName))
	for _, entry := range byName {
		merged = append(merged, entry)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Name() < merged[j].Name() })
	return merged
}

func join(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}
