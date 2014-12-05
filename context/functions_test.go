package context

import (
	"bytes"
	"io"
	"testing"

	"github.com/drewwells/spritewell"
)

func wrapCallback(sc SassCallback, ch chan UnionSassValue) SassCallback {
	return func(c *Context, usv UnionSassValue) UnionSassValue {
		usv = sc(c, usv)
		ch <- usv
		return usv
	}
}

func setupCtx(r io.Reader, out io.Writer, cookies ...Cookie) (Context, UnionSassValue, error) {
	ctx := Context{
		Sprites:      make(map[string]spritewell.ImageList),
		OutputStyle:  NESTED_STYLE,
		IncludePaths: make([]string, 0),
		BuildDir:     "test/build",
		ImageDir:     "test/img",
		GenImgDir:    "test/build/img",
		Out:          "",
	}
	var cc chan UnionSassValue
	// If callbacks were made, add them to the context
	// and create channels for communicating with them.
	if len(cookies) > 0 {
		cs := make([]Cookie, len(cookies))
		for i, c := range cookies {
			cs[i] = Cookie{
				c.sign,
				wrapCallback(c.fn, cc),
				&ctx,
			}
		}
	}
	err := ctx.Compile(r, out)
	var usv UnionSassValue
	//usv := <-cc
	return ctx, usv, err
}

func TestFuncImageURL(t *testing.T) {
	ctx := Context{
		BuildDir: "test/build",
		ImageDir: "test/img",
	}

	usv := testMarshal(t, []string{"image.png"})
	usv = ImageURL(&ctx, usv)
	var path string
	Unmarshal(usv, &path)

	if e := "../img/image.png"; e != path {
		t.Errorf("got: %s wanted: %s", path, e)
	}
}

func TestFuncSpriteMap(t *testing.T) {
	ctx := NewContext()
	ctx.BuildDir = "test/build"
	ctx.GenImgDir = "test/build/img"
	ctx.ImageDir = "test/img"

	// Add real arguments when sass lists can be [un]marshalled
	lst := []interface{}{"*.png", float64(5), float64(0)}
	usv := testMarshal(t, lst)
	usv = SpriteMap(ctx, usv)
	var path string
	err := Unmarshal(usv, &path)
	if err != nil {
		t.Error(err)
	}

	if e := "test/build/img/image-8121ae.png"; e != path {
		t.Errorf("got: %s wanted: %s", path, e)
	}
}

func TestCompileSpriteMap(t *testing.T) {
	in := bytes.NewBufferString(`
$aritymap: sprite-map("*.png",1,2); // Optional arguments
$map: sprite-map("*.png"); // One argument
div {
width: $map;
height: $aritymap;
}`)

	ctx := NewContext()

	ctx.BuildDir = "test/build"
	ctx.GenImgDir = "test/build/img"
	ctx.ImageDir = "test/img"
	var out bytes.Buffer
	err := ctx.Compile(in, &out)
	if err != nil {
		t.Error(err)
	}
	exp := `div {
  width: test/build/img/image-8121ae.png;
  height: test/build/img/image-8121ae.png; }
`

	if exp != out.String() {
		t.Errorf("got:\n%s\nwanted:\n%s", out.String(), exp)
	}
}

func TestFuncImageHeight(t *testing.T) {
	in := bytes.NewBufferString(`
$map: sprite-map("*.png",0,0);
div {
    height: image-height($map, "139");
}`)
	var out bytes.Buffer
	setupCtx(in, &out)

	e := `div {
  height: 139px; }
`
	if e != out.String() {
		t.Errorf("got:\n%s\nwanted:\n%s", out.String(), e)
	}
}
