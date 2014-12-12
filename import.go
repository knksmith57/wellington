package sprite_sass

import (
	"errors"
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

func init() {
	// We should programmatically set this based on the input
	files.M = make(map[string]*[]byte, 250)
}

func (p *Parser) ImportPath(dir, file string, mainfile string, partialMap *SafePartialMap) (string, string, error) {

	var fpath string
	baseerr := ""
	//Load and retrieve all tokens from imported file
	path, _ := filepath.Abs(fmt.Sprintf("%s/%s.scss", dir, file))
	pwd := filepath.Dir(path)
	baseerr += fpath + "\n"

	// Look through the import path for the file
	for _, lib := range p.Includes {
		path, _ := filepath.Abs(lib + "/" + file)
		pwd := filepath.Dir(path)
		fpath = filepath.Join(pwd, "/_"+filepath.Base(path)+".scss")
		if cache, ok := files.Read(fpath); ok {
			return pwd, string(cache), nil
		}
		contents, err := ioutil.ReadFile(fpath)
		baseerr += fpath + "\n"
		if err == nil {
			partialMap.AddRelation(mainfile, fpath)
			files.Write(fpath, &contents)
			return pwd, string(contents), nil
		} else {
			// Attempt invalid name lookup (no _)
			fpath = filepath.Join(pwd, "/"+filepath.Base(path)+".scss")
			if cache, ok := files.Read(fpath); ok {
				return pwd, string(cache), nil
			}
			contents, err = ioutil.ReadFile(fpath)
			baseerr += fpath + "\n"
			if err == nil {
				partialMap.AddRelation(mainfile, fpath)
				files.Write(fpath, &contents)
				return pwd, string(contents), nil
			}
		}
	}

	// Check pwd
	// Sass put _ in front of imported files
	fpath = filepath.Join(pwd, "/_"+filepath.Base(path))
	contents, err := ioutil.ReadFile(fpath)
	if err == nil {
		partialMap.AddRelation(mainfile, fpath)
		files.Write(fpath, &contents)
		if cache, ok := files.Read(fpath); ok {
			return pwd, string(cache), nil
		}
		return pwd, string(contents), nil
	}

	// Ignore failures on compass
	re := regexp.MustCompile("compass\\/?")
	if re.Match([]byte(file)) {
		return pwd, string(contents), nil //errors.New("compass")
	}
	if file == "images" {
		return pwd, string(contents), nil
	}
	return pwd, string(contents), errors.New("Could not import: " +
		file + "\nTried:\n" + baseerr)
}
