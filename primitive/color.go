package primitive

import (
	"fmt"
	"image/color"
	"strings"
)

type Color struct {
	R, G, B, A int
}

func MakeColor(c color.Color) *Color {
	r, g, b, a := c.RGBA()
	return &Color{int(r / 257), int(g / 257), int(b / 257), int(a / 257)}
}

func MakeHexColor(x string) (*Color, error) {
	x = strings.Trim(x, "#")
	var r, g, b, a int
	a = 255
	switch len(x) {
	case 3:
		if _, err := fmt.Sscanf(x, "%1x%1x%1x", &r, &g, &b); err != nil {
			return nil, err
		}
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
	case 4:
		if _, err := fmt.Sscanf(x, "%1x%1x%1x%1x", &r, &g, &b, &a); err != nil {
			return nil, err
		}
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
		a = (a << 4) | a
	case 6:
		if _, err := fmt.Sscanf(x, "%02x%02x%02x", &r, &g, &b); err != nil {
			return nil, err
		}
	case 8:
		if _, err := fmt.Sscanf(x, "%02x%02x%02x%02x", &r, &g, &b, &a); err != nil {
			return nil, err
		}
	}
	return &Color{r, g, b, a}, nil
}

func (c *Color) NRGBA() color.NRGBA {
	return color.NRGBA{uint8(c.R), uint8(c.G), uint8(c.B), uint8(c.A)}
}
