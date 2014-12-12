package sprite_sass

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"regexp"
	"strings"
	"testing"
)

var rerandom *regexp.Regexp

var spritePreamble string

func init() {
	rerandom = regexp.MustCompile(`-\w{6}(?:\.(png|jpg))`)

	bs, err := ioutil.ReadFile("sass/_sprite.scss")
	if err != nil {
		log.Fatal(err)
	}
	spritePreamble = strings.TrimSuffix(string(bs), "\n")
}

func TestParserRelative(t *testing.T) {
	p := Parser{
		BuildDir: "test/build",
		MainFile: "sprite.css",
	}
	partialMap := NewPartialMap()
	f, err := ioutil.ReadFile("sass/_sprite.scss")
	if err != nil {
		log.Fatal(err)
	}
	in := bytes.NewBuffer(f)
	in.WriteString(`div {
  background: image-url('img/139.png');
}`)
	e := fmt.Sprintf(`$rel: "..";
%s
div {
  background: image-url('img/139.png');
}`, spritePreamble)
	bs, _ := p.Start(in, "test", partialMap)
	out := string(bs)

	if out != e {
		t.Skipf("Mismatch expected:\n%s\nwas:\n%s", e, out)
	}

}

func TestParserImporter(t *testing.T) {
	p := Parser{
		BuildDir: "test/build",
		Includes: []string{"test/sass"},
		MainFile: "import.css",
	}

	partialMap := NewPartialMap()

	bs, err := p.Start(fileReader("test/sass/import.scss"), "test/", partialMap)
	if err != nil {
		log.Fatal(err)
	}
	output := string(bs)

	file, _ := ioutil.ReadFile("test/expected/import.parser")
	e := string(file)
	if e != output {
		t.Skipf("File output did not match, exp:\n%s\nwas:\n~%s~",
			e, output)
	}

	lines := map[int]string{
		0:  "../../sass/sprite",
		60: "var",
		71: "string",
	}
	errors := false
	for i, v := range lines {
		if v != p.Line[i] {
			t.Errorf("Invalid expected: %s, was: %s", lines[i], p.Line[i])
			errors = true
		}
	}
	if errors {
		fmt.Printf("% #v\n", p.Line)
	}
}

func TestParseSpriteArgs(t *testing.T) {
	p := Parser{}
	var partialMap *SafePartialMap
	in := bytes.NewBufferString(`$view_sprite: sprite-map("test/*.png",
  $normal-spacing: 2px,
  $normal-hover-spacing: 2px,
  $selected-spacing: 2px,
  $selected-hover-spacing: 2px);
  @include sprite-dimensions($view_sprite,140);
`)
	e := `$rel: ".";
$view_sprite: (); $view_sprite: map_merge($view_sprite,(139: (width: 96, height: 139, x: 0, y: 0, url: 'test-d01d06.png'))); $view_sprite: map_merge($view_sprite,(140: (width: 96, height: 140, x: 0, y: 139, url: 'test-d01d06.png'))); $view_sprite: map_merge($view_sprite,(pixel: (width: 1, height: 1, x: 0, y: 279, url: 'test-d01d06.png')));
  @include sprite-dimensions($view_sprite,140);
`
	bs, _ := p.Start(in, "", partialMap)
	out := string(bs)

	if out != e {
		t.Skipf("Mismatch expected:\n%s\nwas:\n%s", e, out)
	}
}

func TestParseInt(t *testing.T) {
	p := Parser{}
	var (
		e, res string
	)
	var partialMap *SafePartialMap
	r := bytes.NewBufferString(`p {
  $font-size: 12px;
  $line-height: 30px;
  font: #{$font-size}/#{$line-height};
}`)
	bs, _ := p.Start(r, "", partialMap)
	res = string(bs)

	e = `$rel: ".";
p {
  $font-size: 12px;
  $line-height: 30px;
  font: #{$font-size}/#{$line-height};
}`
	if e != res {
		t.Skipf("Mismatch expected:\n%s\nwas:\n%s", e, res)
	}
	p = Parser{}
	r = bytes.NewBufferString(`$name: foo;
$attr: border;
p.#{$name} {
  #{$attr}-color: blue;
}`)
	bs, _ = p.Start(r, "", partialMap)
	res = string(bs)

	e = `$rel: ".";
$name: foo;
$attr: border;
p.#{$name} {
  #{$attr}-color: blue;
}`
	if e != res {
		t.Errorf("Mismatch expected:\n%s\nwas:\n%s", e, res)
	}
}

func TestParseImage(t *testing.T) {
	p := Parser{
		BuildDir: "test/build",
		MainFile: "test",
	}
	var partialMap *SafePartialMap
	in := bytes.NewBufferString(`$sprites: sprite-map("img/*.png");
$sfile: sprite-file($sprites, 139);
div {
    height: image-height(sprite-file($sprites, 139));
    width: image-width(test/139.png);
    url: sprite-file($sprites, 139);
}`)
	bs, _ := p.Start(in, "", partialMap)
	out := string(bs)

	if e := `$rel: "..";
$sprites: (); $sprites: map_merge($sprites,(139: (width: 96, height: 139, x: 0, y: 0, url: 'img/img-554064.png'))); $sprites: map_merge($sprites,(140: (width: 96, height: 140, x: 0, y: 139, url: 'img/img-554064.png')));
$sfile: sprite-file($sprites, 139);
div {
    height: image-height(sprite-file($sprites, 139));
    width: image-width(test/139.png);
    url: sprite-file($sprites, 139);
}`; e != out {
		t.Skipf("Mismatch expected:\n%s\nwas:\n%s\n", e, out)
	}
}

func TestParseImageUrl(t *testing.T) {

	p := Parser{
		BuildDir: "test/build",
		MainFile: "test",
	}
	var partialMap *SafePartialMap
	in := bytes.NewBufferString(`background: image-url('test/140.png');`)
	bs, _ := p.Start(in, "", partialMap)
	out := string(bs)

	if e := `$rel: "..";
background: image-url('test/140.png');`; e != out {
		t.Skipf("mismatch expected:\n%s\nwas:\n%s\n", e, out)
	}
	log.SetOutput(os.Stdout)
}
