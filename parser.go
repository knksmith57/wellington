package sprite_sass

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
)

/* Example sprite-map output:
$sprites: ($rel: "");

$sprites: map_merge($sprites, (
  139: (
    width: 139,
    height: 89,
    x: 0,
    y: 20,
    url: './image.png'
  )));

$sprites: map_merge($sprites,(140: (
    width: 140,
    height: 89,
    x: 0,
    y: 20,
    url: './image.png'
  )));
*/

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
}

type Replace struct {
	Start, End int
	Value      []byte
}

type Parser struct {
	Idx, shift           int
	Chop                 []Replace
	Pwd, Input, MainFile string
	SassDir, BuildDir,
	GenImgDir string
	StaticDir           string
	ProjDir             string
	ImageDir            string
	Includes            []string
	Items               []Item
	Output              []byte
	Line                map[int]string
	LineKeys            []int
	InlineImgs, Sprites map[string]ImageList
	Vars                map[string]string
}

func NewParser() *Parser {
	return &Parser{}
}

// Parser reads the tokens from the lexer and performs
// conversions and/or substitutions for sprite*() calls.
//
// Parser creates a map of all variables and sprites
// (created via sprite-map calls).
func (p *Parser) Start(in io.Reader, pkgdir string) ([]byte, error) {
	p.Vars = make(map[string]string)
	p.Sprites = make(map[string]ImageList)
	p.InlineImgs = make(map[string]ImageList)
	p.Line = make(map[int]string)

	// Setup paths
	if p.MainFile == "" {
		p.MainFile = "string"
	}
	if p.BuildDir == "" {
		p.BuildDir = pkgdir
	}
	if p.SassDir == "" {
		p.SassDir = pkgdir
	}
	if p.StaticDir == "" {
		p.StaticDir = pkgdir
	}
	if p.ImageDir == "" {
		p.ImageDir = p.StaticDir
	}
	if p.GenImgDir == "" {
		p.GenImgDir = p.BuildDir
	}
	buf := bytes.NewBuffer(make([]byte, 0, bytes.MinRead))
	buf.ReadFrom(in)

	// This pass resolves all the imports, but positions will
	// be off due to @import calls
	items, input, err := p.GetItems(pkgdir, p.MainFile, string(buf.Bytes()))
	if err != nil {
		return []byte(""), err
	}
	for i := range p.Line {
		p.LineKeys = append(p.LineKeys, i)
	}
	sort.Ints(p.LineKeys)
	// This call will have valid token positions
	items, input, err = p.GetItems(pkgdir, p.MainFile, input)
	p.Input = input
	p.Items = items
	if err != nil {
		panic(err)
	}
	// DEBUG
	// for _, item := range p.Items {
	// 	fmt.Printf("%s %s\n", item.Type, item)
	// }
	// Process sprite calls and gen
	p.Parse(p.Items)
	p.Output = []byte(p.Input)
	// Perform substitutions
	p.Replace()
	rel := []byte(fmt.Sprintf(`$rel: "%s";%s`, p.Rel(), "\n"))

	return append(rel, p.Output...), nil
}

func (p *Parser) Rel() string {
	rel, _ := filepath.Rel(p.BuildDir, p.StaticDir)
	return filepath.Clean(rel)
}

// LookupFile translates line positions into line number
// and file it belongs to
func (p *Parser) LookupFile(pos int) string {
	pos = pos - 1
	for i, n := range p.LineKeys {
		if n > pos {
			if i == 0 {
				return fmt.Sprintf("%s:%d", p.MainFile, pos+1)
			}
			hit := p.LineKeys[i-1]
			return fmt.Sprintf("%s:%d", p.Line[hit], pos-p.LineKeys[i-1])
		}
	}
	return "mainfile?" + p.MainFile
}

// Find Paren that matches the current (
func RParen(items []Item) (int, int) {
	if len(items) == 0 {
		return 0, 0
	}
	if items[0].Type != LPAREN {
		panic("Expected: ( was: " + items[0].Value)
	}
	pos := 1
	match := 1
	nest := false
	nestPos := 0

	for match != 0 && pos < len(items) {
		switch items[pos].Type {
		case LPAREN:
			match++
		case RPAREN:
			match--
		}
		if match > 1 {
			if !nest {
				nestPos = pos
			}
			// Nested command must be resolved
			nest = true
		}
		pos++
	}

	return pos, nestPos
}

func RBracket(items []Item, pos int) (int, int) {
	if items[pos].Type != LBRACKET && items[pos].Type != INTP {
		panic("Expected: { was: " + items[0].Value)
	}

	// Move to next item and set match to 1
	pos++
	match := 1
	nest := false
	nestPos := 0
	for match != 0 && pos < len(items) {
		switch items[pos].Type {
		case LBRACKET, INTP:
			match++
		case RBRACKET:
			match--
		}
		if match > 1 {
			if !nest {
				nestPos = pos
			}
			// Nested command must be resolved
			nest = true
		}
		pos++
	}
	return pos, nestPos
}

func (p *Parser) Parse(items []Item) []byte {
	var (
		out []byte
		eoc int
	)
	_ = eoc
	if len(items) == 0 {
		return []byte("")
	}
	j := 1
	item := items[0]
	switch item.Type {
	case VAR:
		if items[1].Value != ":" {
			log.Fatal(": expected after variable declaration")
		}
		for j < len(items) && items[j].Type != SEMIC {
			j++
		}
		if items[2].Type != CMDVAR {
			// Hackery for empty sass maps
			val := string(p.Parse(items[2:j]))
			// TODO: $var: $anothervar doesnt work
			// setting other things like $var: darken(#123, 10%)
			if val != "()" && val != "" {
				// fmt.Println("SETTING", item, val)
				p.Vars[item.String()] = val
			}
		} else if items[2].Value == "sprite-map" {
			// Special parsing of sprite-maps
			imgs := ImageList{
				ImageDir:  p.ImageDir,
				BuildDir:  p.BuildDir,
				GenImgDir: p.GenImgDir,
			}
			name := fmt.Sprintf("%s", items[0])
			glob := fmt.Sprintf("%s", items[4])
			imgs.Decode(glob)
			imgs.Vertical = true
			imgs.Combine()
			p.Sprites[name] = imgs
			//TODO: Generate filename
			p.Mark(items[2].Pos,
				items[j].Pos+len(items[j].Value), imgs.Map(name))
			_, err := imgs.Export()
			if err != nil {
				log.Printf("Failed to save sprite: %s", name)
				log.Println(err)
			}
		}
	}

	return append(out, p.Parse(items[j:])...)
}

// Deprecated
// Passed sass-command( args...)
func (p *Parser) Command(items []Item) ([]byte, int) {

	i := 0
	_ = i
	cmd := items[0]
	repl := ""
	if len(items) == 0 {
		panic(items)
	}
	eoc, nPos := RParen(items[1:])
	// Determine our offset from the source items
	if false && nPos != 0 {
		rightPos, _ := RParen(items[nPos:])
		p.Command(items[nPos:rightPos])
	}
	return []byte(""), eoc
	switch cmd.Value {
	case "sprite":
		//Capture sprite
		sprite := p.Sprites[fmt.Sprintf("%s", items[2])]
		pos, _ := RParen(items[1:])
		//Capture filename
		name := fmt.Sprintf("%s", items[3])
		repl = sprite.CSS(name)
		p.Mark(items[0].Pos, items[pos].Pos+len(items[pos].Value), repl)
	case "sprite-height":
		sprite := p.Sprites[fmt.Sprintf("%s", items[2])]
		repl = fmt.Sprintf("%dpx", sprite.SImageHeight(items[3].String()))
		p.Mark(cmd.Pos, items[eoc].Pos+len(items[eoc].Value), repl)
	case "sprite-width":
		sprite := p.Sprites[fmt.Sprintf("%s", items[2])]
		repl = fmt.Sprintf("%dpx",
			sprite.SImageWidth(items[3].String()))
		p.Mark(cmd.Pos, items[eoc].Pos+len(items[eoc].Value), repl)
	case "sprite-dimensions":
		sprite := p.Sprites[fmt.Sprintf("%s", items[2])]
		repl = sprite.Dimensions(items[3].Value)
		p.Mark(items[0].Pos, items[4].Pos+len(items[4].Value), repl)
	case "sprite-file":
		if items[2].Type != SUB {
			log.Fatalf("%s must be followed by variable, was: %s",
				cmd.Value, items[2].Value)
		}
		if items[3].Type != FILE {
			log.Fatalf("sprite-file must be followed by "+
				"sprite-variable, was: %s",
				items[3].Type)
		}
		repl := p.Sprites[fmt.Sprintf("%s", items[2])].
			File(items[3].String())
		p.Mark(items[0].Pos, items[4].Pos+len(items[4].Value), repl)
		return []byte(repl), eoc
	case "image-height", "image-width":
		if items[2].Type == FILE {
			name := items[2].Value
			img := ImageList{
				ImageDir:  p.ImageDir,
				BuildDir:  p.BuildDir,
				GenImgDir: p.GenImgDir,
			}
			img.Decode(name)
			var d int
			if cmd.Value == "image-width" {
				d = img.ImageWidth(0)
			} else if cmd.Value == "image-height" {
				d = img.ImageHeight(0)
			}
			repl = fmt.Sprintf("%dpx", d)
			p.Mark(items[0].Pos, items[3].Pos+len(items[3].Value), repl)
			return []byte(repl), eoc
		}
		if items[2].Type != CMD {
			log.Fatalf("%s first arg must be sprite-file, was: %s",
				cmd.Value, items[2].Value)
		}
		if items[4].Type != SUB {
			log.Fatalf("%s must be followed by variable, was: %s",
				cmd.Value, items[4].Type)
		}
		// Resolve variable
		sprite := p.Sprites[items[4].Value]
		var pix int
		if cmd.Value == "image-width" {
			pix = sprite.SImageWidth(items[5].Value)
		} else if cmd.Value == "image-height" {
			pix = sprite.SImageHeight(items[5].Value)
		}
		repl := fmt.Sprintf("%dpx", pix)
		p.Mark(items[0].Pos, items[7].Pos+len(items[6].Value), repl)
	case "inline-image":
		var (
			img ImageList
			ok  bool
		)
		name := fmt.Sprintf("%s", items[2])
		if img, ok = p.InlineImgs[name]; !ok {
			img = ImageList{
				ImageDir:  p.ImageDir,
				BuildDir:  p.BuildDir,
				GenImgDir: p.GenImgDir,
			}
			img.Decode(name)
			img.Combine()
			_, err := img.Export()
			if err != nil {
				log.Printf("Failed to save sprite: %s", name)
				log.Println(err)
			}
			p.InlineImgs[name] = img
		}

		repl := img.Inline()
		p.Mark(items[0].Pos, items[3].Pos+len(items[3].Value), repl)
	default:
		fmt.Println("No comprende:", items[0])
	}

	return []byte(""), eoc
}

// Import recursively resolves all imports.  It lexes the input
// adding the tokens to the Parser object.
// TODO: Convert this to byte slice in/out
func (p *Parser) GetItems(pwd, filename, input string) ([]Item, string, error) {

	var (
		status    []Item
		importing bool
		output    []byte
		pos       int
		last      *Item
		lastname  string
		lineCount int
	)

	lex := New(func(lex *Lexer) StateFn {
		return lex.Action()
	}, input)

	for {
		item := lex.Next()
		err := item.Error()
		//fmt.Println(item.Type, item.Value)
		if err != nil {
			return nil, string(output),
				fmt.Errorf("Error: %v (pos %d)", err, item.Pos)
		}
		switch item.Type {
		case ItemEOF:
			output = append(output, input[pos:]...)
			return status, string(output), nil
		case IMPORT:
			output = append(output, input[pos:item.Pos]...)
			last = item
			importing = true
		case INCLUDE, CMT:
			output = append(output, input[pos:item.Pos]...)
			pos = item.Pos
			status = append(status, *item)
		default:
			if importing {
				lastname = filename
				filename = fmt.Sprintf("%s", *item)
				for _, nl := range output {
					if nl == '\n' {
						lineCount++
					}
				}
				p.Line[lineCount] = filename
				pwd, contents, err := p.ImportPath(pwd, filename)
				if err != nil {
					return nil, "", err
				}
				//Eat the semicolon
				item := lex.Next()
				pos = item.Pos + len(item.Value)
				if item.Type != SEMIC {
					panic("@import statement must be followed by ;")
				}

				moreTokens, moreOutput, err := p.GetItems(
					pwd,
					filename,
					contents)
				// If importing was successful, each token must be moved
				// forward by the position of the @import call that made
				// it available.
				for i, _ := range moreTokens {
					moreTokens[i].Pos += last.Pos
				}

				if err != nil {
					return nil, "", err
				}
				for _, nl := range moreOutput {
					if nl == '\n' {
						lineCount++
					}
				}
				filename = lastname
				p.Line[lineCount+1] = filename
				output = append(output, moreOutput...)
				status = append(status, moreTokens...)
				importing = false
			} else {
				output = append(output, input[pos:item.Pos]...)
				pos = item.Pos
				status = append(status, *item)
			}
		}
	}
}
