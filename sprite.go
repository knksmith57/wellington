package sprite_sass

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"image/draw"
	_ "image/jpeg"
	"image/png"
)

type GoImages []image.Image
type ImageList struct {
	GoImages
	BuildDir, ImageDir, GenImgDir string
	Out                           draw.Image
	OutFile                       string
	Combined                      bool
	Files                         []string
	Vertical                      bool
}

func (l ImageList) String() string {
	files := ""
	for _, file := range l.Files {
		files += strings.TrimSuffix(filepath.Base(file),
			filepath.Ext(file)) + " "
	}
	return files
}

func (l ImageList) Lookup(f string) int {
	var base string

	for i, v := range l.Files {
		base = filepath.Base(v)
		base = strings.TrimSuffix(base, filepath.Ext(v))
		if f == v {
			return i
			//Do partial matches, for now
		} else if f == base {
			return i
		}
	}
	// TODO: Find a better way to send these to cli so tests
	// aren't impacted.
	// Debug.Printf("File not found: %s\n Try one of %s", f, l)

	return -1
}

// Return the X position of an image based
// on the layout (vertical/horizontal) and
// position in Image slice
func (l ImageList) X(pos int) int {
	x := 0
	if l.Vertical {
		return 0
	}
	for i := 0; i < pos; i++ {
		x += l.ImageWidth(i)
	}
	return x
}

// Return the Y position of an image based
// on the layout (vertical/horizontal) and
// position in Image slice
func (l ImageList) Y(pos int) int {
	y := 0
	if !l.Vertical {
		return 0
	}
	if pos > len(l.GoImages) {
		return -1
	}
	for i := 0; i < pos; i++ {
		y += l.ImageHeight(i)
	}
	return y
}

func (l ImageList) CSS(s string) string {
	pos := l.Lookup(s)
	if pos == -1 {
		log.Printf("File not found: %s\n Try one of: %s",
			s, l)
	}

	return fmt.Sprintf(`url("%s") %s`,
		l.OutFile, l.Position(s))
}

func (l ImageList) Position(s string) string {
	pos := l.Lookup(s)
	if pos == -1 {
		log.Printf("File not found: %s\n Try one of: %s",
			s, l)
	}

	return fmt.Sprintf(`%dpx %dpx`, -l.X(pos), -l.Y(pos))
}

func (l ImageList) Dimensions(s string) string {
	if pos := l.Lookup(s); pos > -1 {

		return fmt.Sprintf("width: %dpx;\nheight: %dpx",
			l.ImageWidth(pos), l.ImageHeight(pos))
	}
	return ""
}

func (l ImageList) inline() []byte {

	r, w := io.Pipe()
	go func(w io.WriteCloser) {
		err := png.Encode(w, l.GoImages[0])
		if err != nil {
			panic(err)
		}
		w.Close()
	}(w)
	var scanned []byte
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		scanned = append(scanned, scanner.Bytes()...)
	}
	return scanned
}

// Inline creates base64 encoded string of the underlying
// image data blog
func (l ImageList) Inline() string {
	encstr := base64.StdEncoding.EncodeToString(l.inline())
	return fmt.Sprintf("url('data:image/png;base64,%s')", encstr)
}

func (l ImageList) SImageWidth(s string) int {
	if pos := l.Lookup(s); pos > -1 {
		return l.ImageWidth(pos)
	}
	return -1
}

func (l ImageList) ImageWidth(pos int) int {
	if pos > len(l.GoImages) {
		return -1
	}
	return l.GoImages[pos].Bounds().Dx()
}

func (l ImageList) SImageHeight(s string) int {
	if pos := l.Lookup(s); pos > -1 {
		return l.ImageHeight(pos)
	}
	return -1
}

func (l ImageList) ImageHeight(pos int) int {
	if pos > len(l.GoImages) {
		return -1
	}
	return l.GoImages[pos].Bounds().Dy()
}

// Return the cumulative Height of the
// image slice.
func (l *ImageList) Height() int {
	h := 0
	ll := *l

	for pos, _ := range ll.GoImages {
		if l.Vertical {
			h += ll.ImageHeight(pos)
		} else {
			h = int(math.Max(float64(h), float64(ll.ImageHeight(pos))))
		}
	}
	return h
}

// Return the cumulative Width of the
// image slice.
func (l *ImageList) Width() int {
	w := 0

	for pos, _ := range l.GoImages {
		if !l.Vertical {
			w += l.ImageWidth(pos)
		} else {
			w = int(math.Max(float64(w), float64(l.ImageWidth(pos))))
		}
	}
	return w
}

// Build an output file location based on
// [genimagedir|location of file matched by glob] + glob pattern
func (l *ImageList) OutputPath(globpath string) {

	gdir, err := filepath.Rel(l.BuildDir, l.GenImgDir)
	if err != nil {
		log.Fatal(err)
	}

	path := filepath.Dir(globpath)
	if path == "." {
		path = "image"
	}
	path = strings.Replace(path, "/", "", -1)
	ext := filepath.Ext(globpath)

	// Remove invalid characters from path
	path = strings.Replace(path, "*", "", -1)
	l.OutFile += gdir + "/" + path + "-" +
		randString(6) + ext
}

// Accept a variable number of image globs appending
// them to the ImageList.
func (l *ImageList) Decode(rest ...string) error {

	// Invalidate the composite cache
	l.Out = nil
	var (
		paths []string
	)
	for _, r := range rest {
		matches, err := filepath.Glob(filepath.Join(l.ImageDir, r))
		if err != nil {
			panic(err)
		}
		paths = append(paths, matches...)
	}
	// Send first glob as definition for output path
	l.OutputPath(rest[0])

	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		goimg, str, err := image.Decode(f)
		_ = str // Image format ie. png
		if err != nil {
			panic(err)
		}
		l.GoImages = append(l.GoImages, goimg)
		l.Files = append(l.Files, path)
	}

	return nil
}

// Combine all images in the slice into a final output
// image.
func (l *ImageList) Combine() {

	var (
		maxW, maxH int
	)

	if l.Out != nil {
		return
	}

	maxW, maxH = l.Width(), l.Height()

	curH, curW := 0, 0

	goimg := image.NewRGBA(image.Rect(0, 0, maxW, maxH))
	l.Out = goimg
	for _, img := range l.GoImages {

		draw.Draw(goimg, goimg.Bounds(), img,
			image.Point{
				X: curW,
				Y: curH,
			}, draw.Src)

		if l.Vertical {
			curH -= img.Bounds().Dy()
		} else {
			curW -= img.Bounds().Dx()
		}
	}

	l.Combined = true
}

func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

// Export saves out the ImageList to the specified file
func (l *ImageList) Export() (string, error) {
	// Use the auto generated path if none is specified

	// TODO: Differentiate relative file path (in css) to this abs one
	abs := filepath.Join(l.GenImgDir, filepath.Base(l.OutFile))
	// Create directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(abs), 0777)
	if err != nil {
		log.Printf("Failed to create image build dir: %s",
			filepath.Dir(abs))
		return "", err
	}
	fo, err := os.Create(abs)
	if err != nil {
		log.Printf("Failed to create file: %s", abs)
		return "", err
	}
	fmt.Println("Created file: ", abs)
	//This call is cached if already run
	l.Combine()

	// Supported compressions http://www.imagemagick.org/RMagick/doc/info.html#compression
	defer fo.Close()

	if err != nil {
		return "", err
	}

	err = png.Encode(fo, l.Out)

	if err != nil {
		panic(err)
		return "", err
	}
	return abs, nil
}
