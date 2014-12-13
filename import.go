package sprite_sass

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sync"
)

// FileCache stores every partial disk I/O in memory. Subsequent
// calls for files will be looked up in the map rather than
// going to disk.
type FileCache struct {
	sync.RWMutex
	M map[string]*[]byte
}

var Miss, Count int

var ImportOnce, ImportRest int

func (f FileCache) Read(key string) ([]byte, bool) {
	f.RLock()
	defer f.RUnlock()
	buf, ok := f.M[key]
	if !ok {
		Miss++
		return nil, ok
	}
	Count++
	return *buf, ok
}

func (f FileCache) Write(key string, bs *[]byte) {
	f.Lock()
	f.M[key] = bs
	f.Unlock()
}

var files FileCache
var re = regexp.MustCompile("compass\\/?")

func init() {
	// We should programmatically set this based on the input
	files.M = make(map[string]*[]byte, 250)
}

// HasImported returns whether or not a partial has been imported.
func (p *Parser) HasImported(key string) bool {
	return false
	for i := range p.Paths {
		if key == p.Paths[i] {
			ImportRest++
			return true
		}
	}
	ImportOnce++
	return false
}

// ImportPath searches through the pwd and include paths looking for
// partials or full sass files.  Compass import failures are discarded.
func (p *Parser) ImportPath(dir, file string, mainfile string, partialMap *SafePartialMap) (string, string, error) {

	pwd, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	// Imports may have directories in them, so extract them
	impDir := filepath.Dir(file)
	impFile := filepath.Base(file)

	// Iterate through the paths building file strings
	fpaths := make([]string, 2, (len(p.Includes)+1)*2)
	fpaths[0] = filepath.Join(pwd, impDir, "_"+impFile+".scss")
	fpaths[1] = filepath.Join(pwd, impDir, impFile+".scss")
	for _, inc := range p.Includes {
		pwd, err := filepath.Abs(inc)
		if err != nil {
			panic(err)
		}
		fpath := filepath.Join(pwd, impDir, "_"+impFile+".scss")
		if fpaths[0] == fpath {
			continue
		}
		fpaths = append(fpaths, fpath)
		fpaths = append(fpaths, filepath.Join(pwd, file+".scss"))
	}

	for _, fp := range fpaths {

		contents, err := ioutil.ReadFile(fp)
		if err == nil {

			// Check if this has already been imported
			if p.HasImported(fp) {
				return pwd, "", nil
			}

			// Look for the file in global cache
			if cache, ok := files.Read(fp); ok {
				return filepath.Dir(fp), string(cache), nil
			}

			partialMap.AddRelation(p.MainFile, fp)
			files.Write(fp, &contents)
			p.Paths = append(p.Paths, fp)
			return filepath.Dir(fp), string(contents), nil
		}
	}

	if re.Match([]byte(file)) {
		return dir, fmt.Sprintf("/* removed @import %s; */", file), nil
	}

	return pwd, "",
		fmt.Errorf("Could not import: %s \nTried:\n%v", file, fpaths)
}
